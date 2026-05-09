package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

type ObjectStore struct {
	pool *pgxpool.Pool
	log  *logging.Logger
}

const objectJSONPreservingManifestCache = `(($5::jsonb - 'manifest' - 'manifest_version') ||
 CASE WHEN objects.json ? 'manifest' THEN jsonb_build_object('manifest', objects.json->'manifest') ELSE '{}'::jsonb END ||
 CASE WHEN objects.json ? 'manifest_version' THEN jsonb_build_object('manifest_version', objects.json->'manifest_version') ELSE '{}'::jsonb END)`

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

func (s *ObjectStore) ListObjects(ctx context.Context, filters ...ports.ObjectFilter) ([]model.Object, error) {
	state := &ports.ObjectFilterState{}
	for _, f := range filters {
		f(state)
	}

	query := `SELECT object_id, type, owner_type, owner_id, json, version, created_at, updated_at FROM objects`
	var conditions []string
	args := make([]any, 0, 4)
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

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY updated_at DESC, object_id ASC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_object_store", "list objects failed", logging.ErrorField(err))
		return nil, fmt.Errorf("list objects: %w", err)
	}
	defer rows.Close()

	var objects []model.Object
	for rows.Next() {
		var o model.Object
		if err := rows.Scan(&o.ObjectID, &o.Type, &o.OwnerType, &o.OwnerID, &o.JSON, &o.Version, &o.CreatedAt, &o.UpdatedAt); err != nil {
			s.log.ErrorContext(ctx, "postgres_object_store", "scan object failed", logging.ErrorField(err))
			return nil, fmt.Errorf("scan object: %w", err)
		}
		objects = append(objects, o)
	}
	return objects, rows.Err()
}

// UpdateObject performs an optimistic-concurrency update on the object's main
// fields (type, owner_type, owner_id, json). Manifest writes go through
// UpdateObjectManifest and do not touch version.
func (s *ObjectStore) UpdateObject(ctx context.Context, obj *model.Object) error {
	if obj == nil {
		return fmt.Errorf("object is nil")
	}
	jsonValue, err := jsonbParam(obj.JSON)
	if err != nil {
		return fmt.Errorf("update object json: %w", err)
	}

	var newVersion int
	err = s.pool.QueryRow(ctx,
		`UPDATE objects SET type=$2, owner_type=$3, owner_id=$4, json=`+objectJSONPreservingManifestCache+`,
		   version = version + 1, updated_at=$6
		 WHERE object_id=$1 AND version=$7
		 RETURNING version`,
		obj.ObjectID, obj.Type, obj.OwnerType, obj.OwnerID, jsonValue, obj.UpdatedAt, obj.Version,
	).Scan(&newVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.classifyMissingUpdate(ctx, obj.ObjectID)
		}
		s.log.ErrorContext(ctx, "postgres_object_store", "update object failed", logging.String("object_id", obj.ObjectID), logging.ErrorField(err))
		return fmt.Errorf("update object: %w", err)
	}
	obj.Version = newVersion
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
   type=$2, owner_type=$3, owner_id=$4, json=`+objectJSONPreservingManifestCache+`,
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

// UpdateObjectManifest writes the manifest cache. It does not bump the row
// version: the manifest is a separate aspect tracked by manifest.Version (a
// content-hash), so manifest writes do not invalidate concurrent UpdateObject
// calls reasoning about the main fields.
func (s *ObjectStore) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest, updatedAt ...time.Time) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	manifest = model.NormalizeManifest(manifest)
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal object manifest: %w", err)
	}
	manifestValue, err := jsonbParam(manifestJSON)
	if err != nil {
		return fmt.Errorf("encode object manifest: %w", err)
	}

	ts := time.Now().UTC()
	if len(updatedAt) > 0 {
		ts = updatedAt[0].UTC()
	}

	tag, err := s.pool.Exec(ctx,
		`UPDATE objects SET json = jsonb_set(jsonb_set(json, '{manifest}', $2::jsonb), '{manifest_version}', to_jsonb($3::text)), updated_at = $4
		 WHERE object_id = $1`,
		objectID, manifestValue, manifest.Version, ts,
	)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_object_store", "update object manifest failed", logging.String("object_id", objectID), logging.ErrorField(err))
		return fmt.Errorf("update object manifest: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (s *ObjectStore) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT json->'manifest' FROM objects WHERE object_id = $1`, objectID).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		s.log.ErrorContext(ctx, "postgres_object_store", "get object manifest failed", logging.String("object_id", objectID), logging.ErrorField(err))
		return nil, fmt.Errorf("get object manifest: %w", err)
	}
	if raw == nil || string(raw) == "null" {
		return model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}), nil
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		s.log.ErrorContext(ctx, "postgres_object_store", "decode object manifest failed", logging.String("object_id", objectID), logging.ErrorField(err))
		return nil, fmt.Errorf("decode object manifest: %w", err)
	}
	return model.NormalizeManifest(&manifest), nil
}

func (s *ObjectStore) classifyMissingUpdate(ctx context.Context, objectID string) error {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM objects WHERE object_id = $1)`, objectID,
	).Scan(&exists); err != nil {
		s.log.ErrorContext(ctx, "postgres_object_store", "classify update miss failed", logging.String("object_id", objectID), logging.ErrorField(err))
		return fmt.Errorf("classify update miss: %w", err)
	}
	if exists {
		return model.ErrVersionConflict
	}
	return model.ErrNotFound
}
