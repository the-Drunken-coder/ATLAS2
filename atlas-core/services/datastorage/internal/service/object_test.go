package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/services/datastorage/internal/objectstorage"
	"github.com/anomalyco/atlas-core/services/datastorage/internal/postgres"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/testsupport"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	cfg := testsupport.TestPostgresConfig()
	testsupport.RequireSafeDatabaseCleanup(t, cfg.PostgresDB)
	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cfg.PostgresDSN())
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	poolCfg.MaxConns = cfg.PostgresMaxConns
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("postgres not available: %v", err)
	}

	log := logging.New(cfg.LogLevel, "atlas-test", "test")
	if err := postgres.InitSchema(ctx, pool, log); err != nil {
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

func TestReconcileObjectsDeletesInvalidObjectFoldersBeforeDBLookup(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	log := logging.New("debug", "atlas-test", "test")
	root := t.TempDir()
	invalidFolder := filepath.Join(root, "bad object id")
	if err := os.Mkdir(invalidFolder, 0o700); err != nil {
		t.Fatalf("create invalid object folder: %v", err)
	}

	objStorage := objectstorage.NewStore(root, log)
	if err := objStorage.InitRoot(); err != nil {
		t.Fatalf("init object storage: %v", err)
	}
	defer objStorage.Close()

	svc := &Service{
		Logger:        log,
		objectStore:   postgres.NewObjectStore(pool, log),
		objectStorage: objStorage,
	}
	if err := svc.ReconcileObjects(context.Background()); err != nil {
		t.Fatalf("reconcile objects: %v", err)
	}
	if _, err := os.Stat(invalidFolder); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected invalid object folder to be deleted, stat err=%v", err)
	}
}
