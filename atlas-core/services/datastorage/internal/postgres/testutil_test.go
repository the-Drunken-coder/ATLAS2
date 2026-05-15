package postgres

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/services/shared/config"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/testsupport"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, cfg := openTestPool(t)
	log := logging.New(cfg.LogLevel, "atlas-test", "test")
	ctx := context.Background()
	if err := InitSchema(ctx, pool, log); err != nil {
		pool.Close()
		t.Fatalf("init schema: %v", err)
	}

	for _, table := range []string{"idempotency_keys", "tasks", "observations", "objects", "entities"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			pool.Close()
			t.Fatalf("cleanup %s: %v", table, err)
		}
	}

	return pool
}

func openTestPool(t *testing.T) (*pgxpool.Pool, *config.Config) {
	t.Helper()

	cfg := testsupport.TestPostgresConfig()
	testsupport.RequireSafeDatabaseCleanup(t, cfg.PostgresDB)

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cfg.PostgresDSN())
	if err != nil {
		t.Fatalf("cannot parse postgres config: %v", err)
	}
	poolCfg.MaxConns = cfg.PostgresMaxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("cannot create postgres pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("postgres not available: %v", err)
	}
	return pool, cfg
}

// assertJSONEqual compares JSON byte slices structurally after unmarshaling so
// JSONB formatting and object key ordering do not affect test assertions.
func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()

	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got JSON failed: %v", err)
	}

	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("unmarshal want JSON failed: %v", err)
	}

	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("expected JSON %s, got %s", string(want), string(got))
	}
}
