package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

type ObservationStore struct {
	pool *pgxpool.Pool
	log  *logging.Logger
}

func NewObservationStore(pool *pgxpool.Pool, logs ...*logging.Logger) *ObservationStore {
	return &ObservationStore{pool: pool, log: loggerOrNop(logs...)}
}

func (s *ObservationStore) CreateObservation(ctx context.Context, obs *model.Observation) error {
	if obs == nil {
		return fmt.Errorf("observation is nil")
	}
	jsonValue, err := jsonbParam(obs.JSON)
	if err != nil {
		return fmt.Errorf("create observation json: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO observations (observation_id, source_asset_id, json, version, created_at, updated_at)
 VALUES ($1, $2, $3::jsonb, 1, $4, $5)`,
		obs.ObservationID, obs.SourceAssetID, jsonValue, obs.CreatedAt, obs.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return model.ErrConflict
		}
		s.log.ErrorContext(ctx, "postgres_observation_store", "create observation failed", logging.String("observation_id", obs.ObservationID), logging.ErrorField(err))
		return fmt.Errorf("create observation: %w", err)
	}
	obs.Version = 1
	return nil
}

func (s *ObservationStore) GetObservation(ctx context.Context, observationID string) (*model.Observation, error) {
	obs := &model.Observation{}
	err := s.pool.QueryRow(ctx,
		`SELECT observation_id, source_asset_id, json, version, created_at, updated_at
 FROM observations WHERE observation_id = $1`, observationID,
	).Scan(&obs.ObservationID, &obs.SourceAssetID, &obs.JSON, &obs.Version, &obs.CreatedAt, &obs.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		s.log.ErrorContext(ctx, "postgres_observation_store", "get observation failed", logging.String("observation_id", observationID), logging.ErrorField(err))
		return nil, fmt.Errorf("get observation: %w", err)
	}
	return obs, nil
}

func (s *ObservationStore) ListObservations(ctx context.Context, filters ...ports.ObservationFilter) ([]model.Observation, error) {
	state := &ports.ObservationFilterState{}
	for _, f := range filters {
		f(state)
	}

	query := `SELECT observation_id, source_asset_id, json, version, created_at, updated_at FROM observations`
	var conditions []string
	args := make([]any, 0, 2)
	argIdx := 1

	if state.SourceAssetID != nil {
		conditions = append(conditions, fmt.Sprintf("source_asset_id = $%d", argIdx))
		args = append(args, *state.SourceAssetID)
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
	query += " ORDER BY updated_at DESC, observation_id ASC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_observation_store", "list observations failed", logging.ErrorField(err))
		return nil, fmt.Errorf("list observations: %w", err)
	}
	defer rows.Close()

	var observations []model.Observation
	for rows.Next() {
		var o model.Observation
		if err := rows.Scan(&o.ObservationID, &o.SourceAssetID, &o.JSON, &o.Version, &o.CreatedAt, &o.UpdatedAt); err != nil {
			s.log.ErrorContext(ctx, "postgres_observation_store", "scan observation failed", logging.ErrorField(err))
			return nil, fmt.Errorf("scan observation: %w", err)
		}
		observations = append(observations, o)
	}
	return observations, rows.Err()
}

// UpdateObservation performs an optimistic-concurrency update.
func (s *ObservationStore) UpdateObservation(ctx context.Context, obs *model.Observation) error {
	if obs == nil {
		return fmt.Errorf("observation is nil")
	}
	jsonValue, err := jsonbParam(obs.JSON)
	if err != nil {
		return fmt.Errorf("update observation json: %w", err)
	}

	// Atomic CTE: attempt the update and classify the miss without a second round-trip.
	var newVersion sql.NullInt64
	var classification string
	err = s.pool.QueryRow(ctx,
		`WITH attempt AS (
		   UPDATE observations SET source_asset_id=$2, json=$3::jsonb,
		     version = version + 1, updated_at=$4
		   WHERE observation_id=$1 AND version=$5
		   RETURNING version
		 ),
		 classification AS (
		   SELECT
		     CASE
		       WHEN EXISTS(SELECT 1 FROM attempt) THEN 'updated'
		       WHEN EXISTS(SELECT 1 FROM observations WHERE observation_id=$1) THEN 'conflict'
		       ELSE 'not_found'
		     END AS result,
		     (SELECT version FROM attempt LIMIT 1) AS ver
		 )
		 SELECT result, ver FROM classification`,
		obs.ObservationID, obs.SourceAssetID, jsonValue, obs.UpdatedAt, obs.Version,
	).Scan(&classification, &newVersion)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_observation_store", "update observation failed", logging.String("observation_id", obs.ObservationID), logging.ErrorField(err))
		return fmt.Errorf("update observation: %w", err)
	}
	switch classification {
	case "updated":
		if !newVersion.Valid {
			return fmt.Errorf("updated observation missing new version")
		}
		obs.Version = int(newVersion.Int64)
		return nil
	case "conflict":
		return model.ErrVersionConflict
	case "not_found":
		return model.ErrNotFound
	default:
		return fmt.Errorf("unexpected classification: %s", classification)
	}
}

func (s *ObservationStore) DeleteObservation(ctx context.Context, observationID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM observations WHERE observation_id = $1`, observationID)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_observation_store", "delete observation failed", logging.String("observation_id", observationID), logging.ErrorField(err))
		return fmt.Errorf("delete observation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

// UpsertObservation is the explicit-clobber escape hatch.
func (s *ObservationStore) UpsertObservation(ctx context.Context, obs *model.Observation) error {
	if obs == nil {
		return fmt.Errorf("observation is nil")
	}
	jsonValue, err := jsonbParam(obs.JSON)
	if err != nil {
		return fmt.Errorf("upsert observation json: %w", err)
	}

	var newVersion int
	err = s.pool.QueryRow(ctx,
		`INSERT INTO observations (observation_id, source_asset_id, json, version, created_at, updated_at)
 VALUES ($1, $2, $3::jsonb, 1, $4, $5)
 ON CONFLICT (observation_id) DO UPDATE SET
   source_asset_id=$2, json=$3::jsonb,
   version = observations.version + 1, updated_at=$5
 RETURNING version`,
		obs.ObservationID, obs.SourceAssetID, jsonValue, obs.CreatedAt, obs.UpdatedAt,
	).Scan(&newVersion)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_observation_store", "upsert observation failed", logging.String("observation_id", obs.ObservationID), logging.ErrorField(err))
		return fmt.Errorf("upsert observation: %w", err)
	}
	obs.Version = newVersion
	return nil
}
