package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/store"
)

// IdempotencyStore is a small dedup table keyed by (scope, key). The scope
// distinguishes operation classes ("object_create", "task_create", etc.) so the
// same client-supplied key can be reused across unrelated mutations without
// collision.
type IdempotencyStore struct {
	pool *pgxpool.Pool
	log  *logging.Logger
}

func NewIdempotencyStore(pool *pgxpool.Pool, logs ...*logging.Logger) *IdempotencyStore {
	return &IdempotencyStore{pool: pool, log: loggerOrNop(logs...)}
}

func (s *IdempotencyStore) TryBegin(ctx context.Context, scope, key, resourceID string) (record store.IdempotencyRecord, claimed bool, err error) {
	if scope == "" {
		return record, false, fmt.Errorf("idempotency scope is required")
	}
	if key == "" {
		return record, false, fmt.Errorf("idempotency key is required")
	}
	if resourceID == "" {
		return record, false, fmt.Errorf("idempotency resource_id is required")
	}

	err = s.pool.QueryRow(ctx,
		`INSERT INTO idempotency_keys (key, scope, resource_id, status)
 VALUES ($1, $2, $3, $4)
 ON CONFLICT (scope, key) DO NOTHING
 RETURNING resource_id, status`,
		key, scope, resourceID, store.IdempotencyStatusPending,
	).Scan(&record.ResourceID, &record.Status)
	if err == nil {
		return record, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		s.log.ErrorContext(ctx, "postgres_idempotency_store", "begin failed",
			logging.String("scope", scope),
			logging.String("key", key),
			logging.ErrorField(err),
		)
		return record, false, fmt.Errorf("begin idempotency key: %w", err)
	}

	err = s.pool.QueryRow(ctx,
		`UPDATE idempotency_keys
		    SET status = $4, resource_id = $3, updated_at = NOW()
		  WHERE scope = $1 AND key = $2 AND resource_id = $3 AND status = $5
		RETURNING resource_id, status`,
		scope, key, resourceID, store.IdempotencyStatusPending, store.IdempotencyStatusFailed,
	).Scan(&record.ResourceID, &record.Status)
	if err == nil {
		return record, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return record, false, fmt.Errorf("reclaim failed idempotency key: %w", err)
	}

	if err := s.pool.QueryRow(ctx,
		`SELECT resource_id, status FROM idempotency_keys WHERE key = $1 AND scope = $2`,
		key, scope,
	).Scan(&record.ResourceID, &record.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return record, false, fmt.Errorf("idempotency key disappeared mid-begin")
		}
		return record, false, fmt.Errorf("read existing idempotency key: %w", err)
	}
	return record, false, nil
}

func (s *IdempotencyStore) MarkCompleted(ctx context.Context, scope, key string) error {
	return s.setStatus(ctx, scope, key, store.IdempotencyStatusCompleted, "")
}

func (s *IdempotencyStore) MarkFailed(ctx context.Context, scope, key string) error {
	return s.setStatus(ctx, scope, key, store.IdempotencyStatusFailed, store.IdempotencyStatusPending)
}

func (s *IdempotencyStore) setStatus(ctx context.Context, scope, key string, status, fromStatus store.IdempotencyStatus) error {
	if scope == "" {
		return fmt.Errorf("idempotency scope is required")
	}
	if key == "" {
		return fmt.Errorf("idempotency key is required")
	}

	query := `UPDATE idempotency_keys SET status = $3, updated_at = NOW() WHERE scope = $1 AND key = $2`
	args := []any{scope, key, status}
	if fromStatus != "" {
		query += ` AND status = $4`
		args = append(args, fromStatus)
	}
	_, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_idempotency_store", "status update failed",
			logging.String("scope", scope),
			logging.String("key", key),
			logging.String("status", string(status)),
			logging.ErrorField(err),
		)
		return fmt.Errorf("update idempotency key status: %w", err)
	}
	return nil
}
