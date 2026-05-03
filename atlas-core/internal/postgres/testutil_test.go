package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/anomalyco/atlas-core/internal/config"
	"github.com/anomalyco/atlas-core/internal/logging"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	cfg := &config.Config{
		PostgresHost:     envOrDefault("ATLAS_TEST_POSTGRES_HOST", "localhost"),
		PostgresPort:     envOrDefault("ATLAS_TEST_POSTGRES_PORT", "5432"),
		PostgresUser:     envOrDefault("ATLAS_TEST_POSTGRES_USER", "atlas"),
		PostgresPassword: envOrDefault("ATLAS_TEST_POSTGRES_PASSWORD", "atlas"),
		PostgresDB:       envOrDefault("ATLAS_TEST_POSTGRES_DB", "atlas_core_test"),
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cfg.PostgresDSN())
	if err != nil {
		t.Skipf("cannot parse postgres config: %v", err)
	}
	poolCfg.MaxConns = 4

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Skipf("cannot create postgres pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("postgres not available: %v", err)
	}

	log := logging.New(cfg, "test")
	if err := InitSchema(ctx, pool, log); err != nil {
		pool.Close()
		t.Fatalf("init schema: %v", err)
	}

	for _, table := range []string{"tasks", "observations", "objects", "entities"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			pool.Close()
			t.Fatalf("cleanup %s: %v", table, err)
		}
	}

	return pool
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
