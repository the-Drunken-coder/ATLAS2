package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/services/shared/listcursor"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type ObjectStore struct {
	pool *pgxpool.Pool
	log  *logging.Logger
}

func NewObjectStore(pool *pgxpool.Pool, logs ...*logging.Logger) *ObjectStore {
	return &ObjectStore{pool: pool, log: loggerOrNop(logs...)}
}

func (s *ObjectStore) CreateObject(ctx context.Context, obj *model.Object) error {
	if obj == nil {
		return fmt.Errorf("object is nil")
	}
	jsonValue, err := jsonbParam(obj.JSON)
	if err != nil {
		return fmt.Errorf("create object json: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO objects (object_id, type, owner_type, owner_id, json, version, created_at, updated_at)
 VALUES ($1, $2, $3, $4, $5::jsonb, 1, $6, $7)`,
		obj.ObjectID, obj.Type, obj.OwnerType, obj.OwnerID, jsonValue, obj.CreatedAt, obj.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return model.ErrConflict
		}
		s.log.ErrorContext(ctx, "postgres_object_store", "create object failed", logging.String("object_id", obj.ObjectID), logging.ErrorField(err))
		return fmt.Errorf("create object: %w", err)
	}
	obj.Version = 1
	return nil
}

func (s *ObjectStore) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	obj := &model.Object{}
	err := s.pool.QueryRow(ctx,
		`SELECT object_id, type, owner_type, owner_id, json, version, created_at, updated_at
 FROM objects WHERE object_id = $1`, objectID,
	).Scan(&obj.ObjectID, &obj.Type, &obj.OwnerType, &obj.OwnerID, &obj.JSON, &obj.Version, &obj.CreatedAt, &obj.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		s.log.ErrorContext(ctx, "postgres_object_store", "get object failed", logging.String("object_id", objectID), logging.ErrorField(err))
		return nil, fmt.Errorf("get object: %w", err)
	}
	return obj, nil
}

func (s *ObjectStore) ListObjects(ctx context.Context, params store.ObjectListParams) (store.ObjectListResult, error) {
	pageSize, err := listcursor.NormalizePageSize(params.PageSize)
	if err != nil {
		return store.ObjectListResult{}, err
	}

	state := &store.ObjectFilterState{}
	for _, f := range params.Filters {
		f(state)
	}

	query := `SELECT object_id, type, owner_type, owner_id, json, version, created_at, updated_at FROM objects`
	var conditions []string
	args := make([]any, 0, 8)
	argIdx := 1

	if state.OwnerType != nil && state.OwnerID != nil {
		conditions = append(conditions, fmt.Sprintf("owner_type = $%d AND owner_id = $%d", argIdx, argIdx+1))
		args = append(args, *state.OwnerType, *state.OwnerID)
		argIdx += 2
	} else if state.OwnerType != nil {
		conditions = append(conditions, fmt.Sprintf("owner_type = $%d", argIdx))
		args = append(args, *state.OwnerType)
		argIdx++
	} else if state.OwnerID != nil {
		conditions = append(conditions, fmt.Sprintf("owner_id = $%d", argIdx))
		args = append(args, *state.OwnerID)
		argIdx++
	}
	if state.ObjectType != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *state.ObjectType)
		argIdx++
	}
	if state.UpdatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("updated_at > $%d", argIdx))
		args = append(args, state.UpdatedAfter.UTC())
		argIdx++
	}
	var paginationErr error
	var syncWatermark *time.Time
	conditions, args, _, syncWatermark, paginationErr = appendListPagination(params.PageToken, params.StrictSnapshot, "object_id", argIdx, conditions, args)
	if paginationErr != nil {
		return store.ObjectListResult{}, paginationErr
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += listOrderLimit(pageSize, "object_id")

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_object_store", "list objects failed", logging.ErrorField(err))
		return store.ObjectListResult{}, fmt.Errorf("list objects: %w", err)
	}
	defer rows.Close()

	var objects []model.Object
	for rows.Next() {
		var o model.Object
		if err := rows.Scan(&o.ObjectID, &o.Type, &o.OwnerType, &o.OwnerID, &o.JSON, &o.Version, &o.CreatedAt, &o.UpdatedAt); err != nil {
			s.log.ErrorContext(ctx, "postgres_object_store", "scan object failed", logging.ErrorField(err))
			return store.ObjectListResult{}, fmt.Errorf("scan object: %w", err)
		}
		objects = append(objects, o)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(ctx, "postgres_object_store", "iterating object list rows failed", logging.ErrorField(err))
		return store.ObjectListResult{}, fmt.Errorf("iterating object list rows: %w", err)
	}

	trimmed, tok, err := trimPage(objects, pageSize, syncWatermark, func(o model.Object) time.Time { return o.UpdatedAt }, func(o model.Object) string { return o.ObjectID })
	if err != nil {
		return store.ObjectListResult{}, err
	}
	return store.ObjectListResult{Objects: trimmed, NextPageToken: tok}, nil
}

// UpdateObject performs an optimistic-concurrency update on the object's main
// fields (type, owner_type, owner_id, json). Object manifests live on the filesystem only.
func (s *ObjectStore) UpdateObject(ctx context.Context, obj *model.Object) error {
	if obj == nil {
		return fmt.Errorf("object is nil")
	}
	jsonValue, err := jsonbParam(obj.JSON)
	if err != nil {
		return fmt.Errorf("update object json: %w", err)
	}

	var newVersion sql.NullInt64
	var classification string
	err = s.pool.QueryRow(ctx,
		`WITH attempt AS (
		   UPDATE objects SET type=$2, owner_type=$3, owner_id=$4, json=$5::jsonb,
		     version = version + 1, updated_at=$6
		   WHERE object_id=$1 AND version=$7
		   RETURNING version
		 ),
		 classification AS (
		   SELECT
		     CASE
		       WHEN EXISTS(SELECT 1 FROM attempt) THEN 'updated'
		       WHEN EXISTS(SELECT 1 FROM objects WHERE object_id=$1) THEN 'conflict'
		       ELSE 'not_found'
		     END AS result,
		     (SELECT version FROM attempt LIMIT 1) AS ver
		 )
		 SELECT result, ver FROM classification`,
		obj.ObjectID, obj.Type, obj.OwnerType, obj.OwnerID, jsonValue, obj.UpdatedAt, obj.Version,
	).Scan(&classification, &newVersion)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_object_store", "update object failed", logging.String("object_id", obj.ObjectID), logging.ErrorField(err))
		return fmt.Errorf("update object: %w", err)
	}
	version, err := versionFromUpdateClassification(classification, newVersion)
	if err != nil {
		return err
	}
	obj.Version = version
	return nil
}

func (s *ObjectStore) DeleteObject(ctx context.Context, objectID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM objects WHERE object_id = $1`, objectID)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_object_store", "delete object failed", logging.String("object_id", objectID), logging.ErrorField(err))
		return fmt.Errorf("delete object: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

// UpsertObject creates an object or updates its main metadata fields.
func (s *ObjectStore) UpsertObject(ctx context.Context, obj *model.Object) error {
	if obj == nil {
		return fmt.Errorf("object is nil")
	}
	jsonValue, err := jsonbParam(obj.JSON)
	if err != nil {
		return fmt.Errorf("upsert object json: %w", err)
	}

	var newVersion int
	err = s.pool.QueryRow(ctx,
		`INSERT INTO objects (object_id, type, owner_type, owner_id, json, version, created_at, updated_at)
 VALUES ($1, $2, $3, $4, $5::jsonb, 1, $6, $7)
 ON CONFLICT (object_id) DO UPDATE SET
   type=$2, owner_type=$3, owner_id=$4, json=$5::jsonb,
   version = objects.version + 1, updated_at=$7
 RETURNING version`,
		obj.ObjectID, obj.Type, obj.OwnerType, obj.OwnerID, jsonValue, obj.CreatedAt, obj.UpdatedAt,
	).Scan(&newVersion)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_object_store", "upsert object failed", logging.String("object_id", obj.ObjectID), logging.ErrorField(err))
		return fmt.Errorf("upsert object: %w", err)
	}
	obj.Version = newVersion
	return nil
}

// UpdateObjectManifest is a no-op: manifests are stored on the filesystem only.
func (s *ObjectStore) UpdateObjectManifest(context.Context, string, *model.ObjectManifest, ...time.Time) error {
	return nil
}

// GetObjectManifest is unsupported at the store layer; read manifests via the object service.
func (s *ObjectStore) GetObjectManifest(context.Context, string) (*model.ObjectManifest, error) {
	return nil, model.ErrNotFound
}
