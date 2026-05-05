package postgres

import (
	"context"
	"testing"

	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/store"
)

func TestIdempotencyStore_TryBeginReclaimsFailedSameResource(t *testing.T) {
	pool, cfg := openTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	log := logging.New(cfg, "test")
	if err := InitSchema(ctx, pool, log); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM idempotency_keys`); err != nil {
		t.Fatalf("cleanup idempotency_keys: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO idempotency_keys (scope, key, resource_id, status) VALUES ('object_create', 'client-1', 'obj_001', 'failed')`,
	); err != nil {
		t.Fatalf("seed failed key: %v", err)
	}

	s := NewIdempotencyStore(pool, log)
	record, claimed, err := s.TryBegin(ctx, "object_create", "client-1", "obj_001")
	if err != nil {
		t.Fatalf("TryBegin failed: %v", err)
	}
	if !claimed {
		t.Fatal("expected failed key to be reclaimed")
	}
	if record.Status != store.IdempotencyStatusPending {
		t.Fatalf("expected reclaimed status pending, got %q", record.Status)
	}
}

func TestIdempotencyStore_TryBeginReturnsPendingExistingClaim(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO idempotency_keys (scope, key, resource_id, status) VALUES ('task_create', 'client-2', 'task_001', 'pending')`,
	); err != nil {
		t.Fatalf("seed pending key: %v", err)
	}

	s := NewIdempotencyStore(pool)
	record, claimed, err := s.TryBegin(ctx, "task_create", "client-2", "task_001")
	if err != nil {
		t.Fatalf("TryBegin failed: %v", err)
	}
	if claimed {
		t.Fatal("expected existing pending key not to be newly claimed")
	}
	if record.Status != store.IdempotencyStatusPending || record.ResourceID != "task_001" {
		t.Fatalf("unexpected record: %+v", record)
	}
}
