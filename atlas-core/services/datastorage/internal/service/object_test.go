package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/services/datastorage/internal/objectstorage"
	"github.com/anomalyco/atlas-core/services/datastorage/internal/postgres"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
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

func newTestObjectService(t *testing.T) (*Service, string) {
	t.Helper()

	pool := testPool(t)
	t.Cleanup(pool.Close)

	log := logging.New("debug", "atlas-test", "test")
	root := t.TempDir()
	objStorage := objectstorage.NewStore(root, log)
	if err := objStorage.InitRoot(); err != nil {
		t.Fatalf("init object storage: %v", err)
	}
	t.Cleanup(func() {
		if err := objStorage.Close(); err != nil {
			t.Errorf("close object storage: %v", err)
		}
	})

	return &Service{
		Logger:        log,
		objectStore:   postgres.NewObjectStore(pool, log),
		objectStorage: objStorage,
	}, root
}

func createTestObject(t *testing.T, svc *Service, objectID string) {
	t.Helper()

	now := time.Now().UTC()
	object := &model.Object{
		ObjectID:  objectID,
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := svc.objectStore.CreateObject(context.Background(), object); err != nil {
		t.Fatalf("create object %s: %v", objectID, err)
	}
}

func TestReconcileObjectsQuarantinesOrphanFoldersWithoutCreatingDBRows(t *testing.T) {
	svc, root := newTestObjectService(t)
	orphanID := "orphan_obj"
	orphanFolder := filepath.Join(root, orphanID)
	if err := os.Mkdir(orphanFolder, 0o700); err != nil {
		t.Fatalf("create orphan folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orphanFolder, "data.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("write orphan file: %v", err)
	}

	if err := svc.ReconcileObjects(context.Background()); err != nil {
		t.Fatalf("reconcile objects: %v", err)
	}

	if _, err := os.Stat(orphanFolder); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected orphan folder to be removed from active location, stat err=%v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read storage root: %v", err)
	}
	foundQuarantine := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".quarantine-"+orphanID+"-") {
			foundQuarantine = true
			break
		}
	}
	if !foundQuarantine {
		t.Fatalf("expected orphan folder to be quarantined, entries=%v", entries)
	}
	if _, err := svc.objectStore.GetObject(context.Background(), orphanID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected no database object for orphan folder, got err=%v", err)
	}
}

func TestReconcileObjectsDeletesInvalidObjectFoldersBeforeDBLookup(t *testing.T) {
	svc, root := newTestObjectService(t)

	invalidFolder := filepath.Join(root, "bad object id")
	if err := os.Mkdir(invalidFolder, 0o700); err != nil {
		t.Fatalf("create invalid object folder: %v", err)
	}

	if err := svc.ReconcileObjects(context.Background()); err != nil {
		t.Fatalf("reconcile objects: %v", err)
	}
	if _, err := os.Stat(invalidFolder); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected invalid object folder to be deleted, stat err=%v", err)
	}
}

func TestReconcileObjectsRepairsManifestForExistingDBRow(t *testing.T) {
	svc, root := newTestObjectService(t)
	objectID := "obj_test"
	createTestObject(t, svc, objectID)
	if err := svc.objectStorage.CreateObjectFolder(objectID); err != nil {
		t.Fatalf("create object folder: %v", err)
	}
	if err := svc.objectStorage.WriteObjectFile(objectID, "data.txt", []byte("payload")); err != nil {
		t.Fatalf("write object file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, objectID, "manifest.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("corrupt manifest file: %v", err)
	}

	if err := svc.ReconcileObjects(context.Background()); err != nil {
		t.Fatalf("reconcile objects: %v", err)
	}

	manifestBytes, err := svc.objectStorage.ReadManifestFile(objectID)
	if err != nil {
		t.Fatalf("read repaired manifest: %v", err)
	}
	if !json.Valid(manifestBytes) {
		t.Fatalf("expected repaired manifest to be valid json, got %q", string(manifestBytes))
	}
	manifest, err := svc.objectStore.GetObjectManifest(context.Background(), objectID)
	if err != nil {
		t.Fatalf("get cached manifest: %v", err)
	}
	info, ok := manifest.Files["data.txt"]
	if !ok {
		t.Fatalf("expected repaired manifest to include data.txt, got %+v", manifest.Files)
	}
	if info.Size != int64(len("payload")) {
		t.Fatalf("expected repaired manifest size %d, got %d", len("payload"), info.Size)
	}
}

func TestReconcileObjectsDoesNotDeleteDatabaseRowsWithoutFolders(t *testing.T) {
	svc, _ := newTestObjectService(t)
	objectID := "db_only_obj"
	createTestObject(t, svc, objectID)

	if err := svc.ReconcileObjects(context.Background()); err != nil {
		t.Fatalf("reconcile objects: %v", err)
	}

	if _, err := svc.objectStore.GetObject(context.Background(), objectID); err != nil {
		t.Fatalf("expected database row to remain, got err=%v", err)
	}
	exists, err := svc.objectStorage.ObjectFolderExists(objectID)
	if err != nil {
		t.Fatalf("check object folder exists: %v", err)
	}
	if exists {
		t.Fatal("expected reconcile not to create a missing object folder")
	}
}

func TestQuarantineOrphanFolderDeletesWhenRenameFails(t *testing.T) {
	svc, root := newTestObjectService(t)
	orphanID := "delete_me"
	orphanFolder := filepath.Join(root, orphanID)
	if err := os.Mkdir(orphanFolder, 0o700); err != nil {
		t.Fatalf("create orphan folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orphanFolder, "data.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("write orphan file: %v", err)
	}

	originalTimestamp := getQuarantineTimestamp
	fixedTimestamp := time.Now().UnixNano()
	getQuarantineTimestamp = func() int64 { return fixedTimestamp }
	defer func() { getQuarantineTimestamp = originalTimestamp }()

	conflictDir := filepath.Join(root, ".quarantine-"+orphanID+"-"+strconv.FormatInt(fixedTimestamp, 10))
	if err := os.Mkdir(conflictDir, 0o700); err != nil {
		t.Fatalf("create conflict dir %s: %v", conflictDir, err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("write conflict file: %v", err)
	}

	svc.quarantineOrphanFolder(context.Background(), orphanID)

	if _, err := os.Stat(orphanFolder); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected orphan folder to be deleted after rename failure, stat err=%v", err)
	}
}
