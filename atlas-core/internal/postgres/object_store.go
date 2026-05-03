package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/store"
)

type ObjectStore struct {
	pool *pgxpool.Pool
}

func NewObjectStore(pool *pgxpool.Pool) *ObjectStore {
	return &ObjectStore{pool: pool}
}

func (s *ObjectStore) CreateObject(ctx context.Context, obj *model.Object) error {
	jsonValue, err := jsonbParam(obj.JSON)
	if err != nil {
		return fmt.Errorf("create object json: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO objects (object_id, type, owner_type, owner_id, json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)`,
		obj.ObjectID, obj.Type, obj.OwnerType, obj.OwnerID,
		jsonValue, obj.CreatedAt, obj.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return model.ErrConflict
		}
		return fmt.Errorf("create object: %w", err)
	}
	return nil
}

func (s *ObjectStore) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	obj := &model.Object{}
	err := s.pool.QueryRow(ctx,
		`SELECT object_id, type, owner_type, owner_id, json, created_at, updated_at
		 FROM objects WHERE object_id = $1`, objectID,
	).Scan(
		&obj.ObjectID, &obj.Type, &obj.OwnerType, &obj.OwnerID,
		&obj.JSON, &obj.CreatedAt, &obj.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("get object: %w", err)
	}
	return obj, nil
}

func (s *ObjectStore) ListObjects(ctx context.Context, filters ...store.ObjectFilter) ([]model.Object, error) {
	state := &store.ObjectFilterState{}
	for _, f := range filters {
		f(state)
	}

	query := `SELECT object_id, type, owner_type, owner_id, json, created_at, updated_at FROM objects`
	var conditions []string
	var args []interface{}
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
		args = append(args, *state.UpdatedAfter)
		argIdx++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY updated_at DESC, object_id ASC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}
	defer rows.Close()

	var objects []model.Object
	for rows.Next() {
		var o model.Object
		if err := rows.Scan(&o.ObjectID, &o.Type, &o.OwnerType, &o.OwnerID,
			&o.JSON, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan object: %w", err)
		}
		objects = append(objects, o)
	}
	return objects, rows.Err()
}

func (s *ObjectStore) UpdateObject(ctx context.Context, obj *model.Object) error {
	jsonValue, err := jsonbParam(obj.JSON)
	if err != nil {
		return fmt.Errorf("update object json: %w", err)
	}

	tag, err := s.pool.Exec(ctx,
		`UPDATE objects SET type=$2, owner_type=$3, owner_id=$4, json=$5::jsonb, updated_at=$6
		 WHERE object_id=$1`,
		obj.ObjectID, obj.Type, obj.OwnerType, obj.OwnerID,
		jsonValue, obj.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update object: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (s *ObjectStore) DeleteObject(ctx context.Context, objectID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM objects WHERE object_id = $1`, objectID,
	)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (s *ObjectStore) UpsertObject(ctx context.Context, obj *model.Object) error {
	jsonValue, err := jsonbParam(obj.JSON)
	if err != nil {
		return fmt.Errorf("upsert object json: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO objects (object_id, type, owner_type, owner_id, json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
		 ON CONFLICT (object_id) DO UPDATE SET
		   type=$2, owner_type=$3, owner_id=$4, json=$5::jsonb, updated_at=$7`,
		obj.ObjectID, obj.Type, obj.OwnerType, obj.OwnerID,
		jsonValue, obj.CreatedAt, obj.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert object: %w", err)
	}
	return nil
}

func (s *ObjectStore) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest) error {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal object manifest: %w", err)
	}
	manifestValue, err := jsonbParam(manifestJSON)
	if err != nil {
		return fmt.Errorf("encode object manifest: %w", err)
	}

	tag, err := s.pool.Exec(ctx,
		`UPDATE objects SET json = jsonb_set(json, '{manifest}', $2::jsonb), updated_at = NOW()
		 WHERE object_id = $1`,
		objectID, manifestValue,
	)
	if err != nil {
		return fmt.Errorf("update object manifest: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (s *ObjectStore) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx,
		`SELECT json->'manifest' FROM objects WHERE object_id = $1`, objectID,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("get object manifest: %w", err)
	}
	if raw == nil {
		return &model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}, nil
	}
	if string(raw) == "null" {
		return &model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}, nil
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("decode object manifest: %w", err)
	}
	if manifest.Files == nil {
		manifest.Files = map[string]model.ObjectFileInfo{}
	}
	return &manifest, nil
}
