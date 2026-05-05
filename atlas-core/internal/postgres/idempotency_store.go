package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/logging"
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

// TryClaim attempts to record an idempotency-key claim. Returns:
//
//   - claimed=true: the (scope, key) pair was new and is now associated with
//     resourceID. The caller should proceed with its operation.
//   - claimed=false: the pair already existed; existingResourceID is the
//     resource_id from the original claim. The caller should compare against
//     resourceID to distinguish a retry (same resource) from key reuse
//     (different resource).
func (s *IdempotencyStore) TryClaim(ctx context.Context, scope, key, resourceID string) (existingResourceID string, claimed bool, err error) {
	if scope == "" {
		return "", false, fmt.Errorf("idempotency scope is required")
	}
	if key == "" {
		return "", false, fmt.Errorf("idempotency key is required")
	}
	if resourceID == "" {
		return "", false, fmt.Errorf("idempotency resource_id is required")
	}

	// Try to insert; RETURNING is empty on conflict.
	var inserted string
	err = s.pool.QueryRow(ctx,
		`INSERT INTO idempotency_keys (key, scope, resource_id)
 VALUES ($1, $2, $3)
 ON CONFLICT (key) DO NOTHING
 RETURNING resource_id`,
		key, scope, resourceID,
	).Scan(&inserted)
	if err == nil {
		return inserted, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		s.log.ErrorContext(ctx, "postgres_idempotency_store", "claim failed",
			logging.String("scope", scope),
			logging.String("key", key),
			logging.ErrorField(err),
		)
		return "", false, fmt.Errorf("claim idempotency key: %w", err)
	}

	// Conflict — fetch the existing claim.
	var stored string
	if err := s.pool.QueryRow(ctx,
		`SELECT resource_id FROM idempotency_keys WHERE key = $1 AND scope = $2`,
		key, scope,
	).Scan(&stored); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Race: another transaction deleted the key between our INSERT
			// attempt and this SELECT. Caller can retry.
			return "", false, fmt.Errorf("idempotency key disappeared mid-claim")
		}
		return "", false, fmt.Errorf("read existing idempotency key: %w", err)
	}
	return stored, false, nil
}

// Release removes a previously claimed idempotency key. Call this on the
// failure path so a retry isn't misinterpreted as a duplicate.
func (s *IdempotencyStore) Release(ctx context.Context, scope, key string) error {
	if scope == "" || key == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`DELETE FROM idempotency_keys WHERE scope = $1 AND key = $2`,
		scope, key,
	)
	if err != nil {
		s.log.ErrorContext(ctx, "postgres_idempotency_store", "release failed",
			logging.String("scope", scope),
			logging.String("key", key),
			logging.ErrorField(err),
		)
		return fmt.Errorf("release idempotency key: %w", err)
	}
	return nil
}
