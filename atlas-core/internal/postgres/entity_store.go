package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/store"
)

type EntityStore struct {
	pool *pgxpool.Pool
}

func NewEntityStore(pool *pgxpool.Pool) *EntityStore {
	return &EntityStore{pool: pool}
}

func (s *EntityStore) CreateEntity(ctx context.Context, entity *model.Entity) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO entities (entity_id, type, subtype, alias, json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entity.EntityID, entity.Type, entity.Subtype, entity.Alias,
		entity.JSON, entity.CreatedAt, entity.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return model.ErrConflict
		}
		return fmt.Errorf("create entity: %w", err)
	}
	return nil
}

func (s *EntityStore) GetEntity(ctx context.Context, entityID string) (*model.Entity, error) {
	entity := &model.Entity{}
	err := s.pool.QueryRow(ctx,
		`SELECT entity_id, type, subtype, alias, json, created_at, updated_at
		 FROM entities WHERE entity_id = $1`, entityID,
	).Scan(
		&entity.EntityID, &entity.Type, &entity.Subtype, &entity.Alias,
		&entity.JSON, &entity.CreatedAt, &entity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("get entity: %w", err)
	}
	return entity, nil
}

func (s *EntityStore) ListEntities(ctx context.Context, filters ...store.EntityFilter) ([]model.Entity, error) {
	state := &store.EntityFilterState{}
	for _, f := range filters {
		f(state)
	}

	query := `SELECT entity_id, type, subtype, alias, json, created_at, updated_at FROM entities`
	var conditions []string
	var args []interface{}
	argIdx := 1

	if state.EntityType != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *state.EntityType)
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
	query += " ORDER BY updated_at DESC, entity_id ASC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	var entities []model.Entity
	for rows.Next() {
		var e model.Entity
		if err := rows.Scan(&e.EntityID, &e.Type, &e.Subtype, &e.Alias,
			&e.JSON, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

func (s *EntityStore) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE entities SET type=$2, subtype=$3, alias=$4, json=$5, updated_at=$6
		 WHERE entity_id=$1`,
		entity.EntityID, entity.Type, entity.Subtype, entity.Alias,
		entity.JSON, entity.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update entity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (s *EntityStore) DeleteEntity(ctx context.Context, entityID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM entities WHERE entity_id = $1`, entityID,
	)
	if err != nil {
		return fmt.Errorf("delete entity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (s *EntityStore) UpsertEntity(ctx context.Context, entity *model.Entity) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO entities (entity_id, type, subtype, alias, json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (entity_id) DO UPDATE SET
		   type=$2, subtype=$3, alias=$4, json=$5, updated_at=$7`,
		entity.EntityID, entity.Type, entity.Subtype, entity.Alias,
		entity.JSON, entity.CreatedAt, entity.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert entity: %w", err)
	}
	return nil
}

func isDuplicateKey(err error) bool {
	return strings.Contains(err.Error(), "duplicate key")
}
