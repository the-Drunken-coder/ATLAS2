package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/services/shared/listcursor"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type EntityStore struct {
	pool *pgxpool.Pool
	log  *logging.Logger
}

func NewEntityStore(pool *pgxpool.Pool, logs ...*logging.Logger) *EntityStore {
	return &EntityStore{pool: pool, log: loggerOrNop(logs...)}
}

func (s *EntityStore) CreateEntity(ctx context.Context, entity *model.Entity) error {
	if entity == nil {
		return fmt.Errorf("entity is nil")
	}
	jsonValue, err := jsonbParam(entity.JSON)
	if err != nil {
		return fmt.Errorf("create entity json: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO entities (entity_id, type, subtype, alias, json, version, created_at, updated_at)
 VALUES ($1, $2, $3, $4, $5::jsonb, 1, $6, $7)`,
		entity.EntityID, entity.Type, entity.Subtype, entity.Alias,
		jsonValue, entity.CreatedAt, entity.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return model.ErrConflict
		}
		s.log.ErrorContext(ctx, "postgres_entity_store", "create entity failed", logging.String("entity_id", entity.EntityID), logging.ErrorField(err))
		return fmt.Errorf("create entity: %w", err)
	}
	entity.Version = 1
	return nil
}

func (s *EntityStore) GetEntity(ctx context.Context, entityID string) (*model.Entity, error) {
	entity := &model.Entity{}
	err := s.pool.QueryRow(ctx,
		`SELECT entity_id, type, subtype, alias, json, version, created_at, updated_at
 FROM entities WHERE entity_id = $1`, entityID,
	).Scan(
		&entity.EntityID, &entity.Type, &entity.Subtype, &entity.Alias,
		&entity.JSON, &entity.Version, &entity.CreatedAt, &entity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		s.log.ErrorContext(ctx, "postgres_entity_store", "get entity failed", logging.String("entity_id", entityID), logging.ErrorField(err))
		return nil, fmt.Errorf("get entity: %w", err)
	}
	return entity, nil
}

func (s *EntityStore) ListEntities(ctx context.Context, params store.EntityListParams) (store.EntityListResult, error) {
	pageSize, err := listcursor.NormalizePageSize(params.PageSize)
	if err != nil {
		return store.EntityListResult{}, err
	}

	state := &store.EntityFilterState{}
	for _, f := range params.Filters {
		f(state)
	}

	query := `SELECT entity_id, type, subtype, alias, json, version, created_at, updated_at FROM entities`
	var conditions []string
	args := make([]any, 0, 6)
	argIdx := 1

	if state.EntityType != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *state.EntityType)
		argIdx++
	}
	if state.UpdatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("updated_at > $%d", argIdx))
		args = append(args, state.UpdatedAfter.UTC())
		argIdx++
	}
	var cursorErr error
	conditions, args, argIdx, cursorErr = appendKeysetCursor(params.PageToken, "entity_id", argIdx, conditions, args)
	if cursorErr != nil {
		return store.EntityListResult{}, cursorErr
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += listOrderLimit(pageSize, "entity_id")

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_entity_store", "list entities failed", logging.ErrorField(err))
		return store.EntityListResult{}, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	var entities []model.Entity
	for rows.Next() {
		var e model.Entity
		if err := rows.Scan(&e.EntityID, &e.Type, &e.Subtype, &e.Alias, &e.JSON, &e.Version, &e.CreatedAt, &e.UpdatedAt); err != nil {
			s.log.ErrorContext(ctx, "postgres_entity_store", "scan entity failed", logging.ErrorField(err))
			return store.EntityListResult{}, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(ctx, "postgres_entity_store", "iterating entity list rows failed", logging.ErrorField(err))
		return store.EntityListResult{}, fmt.Errorf("iterating entity list rows: %w", err)
	}

	trimmed, tok, err := trimPage(entities, pageSize, func(e model.Entity) time.Time { return e.UpdatedAt }, func(e model.Entity) string { return e.EntityID })
	if err != nil {
		return store.EntityListResult{}, err
	}
	return store.EntityListResult{Entities: trimmed, NextPageToken: tok}, nil
}

// UpdateEntity performs an optimistic-concurrency update. The caller must
// supply the version it last observed in entity.Version; the update succeeds
// only if the database row still has that version. Returns ErrVersionConflict
// when the row exists but the version has moved on, ErrNotFound when the row
// is gone.
func (s *EntityStore) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	if entity == nil {
		return fmt.Errorf("entity is nil")
	}
	jsonValue, err := jsonbParam(entity.JSON)
	if err != nil {
		return fmt.Errorf("update entity json: %w", err)
	}

	var newVersion sql.NullInt64
	var classification string
	err = s.pool.QueryRow(ctx,
		`WITH attempt AS (
		   UPDATE entities SET type=$2, subtype=$3, alias=$4, json=$5::jsonb,
		     version = version + 1, updated_at=$6
		   WHERE entity_id=$1 AND version=$7
		   RETURNING version
		 ),
		 classification AS (
		   SELECT
		     CASE
		       WHEN EXISTS(SELECT 1 FROM attempt) THEN 'updated'
		       WHEN EXISTS(SELECT 1 FROM entities WHERE entity_id=$1) THEN 'conflict'
		       ELSE 'not_found'
		     END AS result,
		     (SELECT version FROM attempt LIMIT 1) AS ver
		 )
		 SELECT result, ver FROM classification`,
		entity.EntityID, entity.Type, entity.Subtype, entity.Alias,
		jsonValue, entity.UpdatedAt, entity.Version,
	).Scan(&classification, &newVersion)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_entity_store", "update entity failed", logging.String("entity_id", entity.EntityID), logging.ErrorField(err))
		return fmt.Errorf("update entity: %w", err)
	}
	version, err := versionFromUpdateClassification(classification, newVersion)
	if err != nil {
		return err
	}
	entity.Version = version
	return nil
}

func (s *EntityStore) DeleteEntity(ctx context.Context, entityID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM entities WHERE entity_id = $1`, entityID)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_entity_store", "delete entity failed", logging.String("entity_id", entityID), logging.ErrorField(err))
		return fmt.Errorf("delete entity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

// UpsertEntity inserts the row if it does not exist and otherwise updates it,
// incrementing the version unconditionally. Use UpdateEntity when you need
// optimistic-concurrency semantics; Upsert is the explicit-clobber escape hatch.
func (s *EntityStore) UpsertEntity(ctx context.Context, entity *model.Entity) error {
	if entity == nil {
		return fmt.Errorf("entity is nil")
	}
	jsonValue, err := jsonbParam(entity.JSON)
	if err != nil {
		return fmt.Errorf("upsert entity json: %w", err)
	}

	var newVersion int
	err = s.pool.QueryRow(ctx,
		`INSERT INTO entities (entity_id, type, subtype, alias, json, version, created_at, updated_at)
 VALUES ($1, $2, $3, $4, $5::jsonb, 1, $6, $7)
 ON CONFLICT (entity_id) DO UPDATE SET
   type=$2, subtype=$3, alias=$4, json=$5::jsonb,
   version = entities.version + 1, updated_at=$7
 RETURNING version`,
		entity.EntityID, entity.Type, entity.Subtype, entity.Alias,
		jsonValue, entity.CreatedAt, entity.UpdatedAt,
	).Scan(&newVersion)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_entity_store", "upsert entity failed", logging.String("entity_id", entity.EntityID), logging.ErrorField(err))
		return fmt.Errorf("upsert entity: %w", err)
	}
	entity.Version = newVersion
	return nil
}

func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
