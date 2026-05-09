package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/adapters/objectstorage"
	"github.com/anomalyco/atlas-core/internal/adapters/postgres"
	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
	"github.com/anomalyco/atlas-core/internal/testsupport"
)

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed.UTC()
}

func testFunctionStores(t *testing.T) (*pgxpool.Pool, *postgres.ObjectStore, *objectstorage.Store, *postgres.IdempotencyStore, *logging.Logger, func()) {
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

	log := logging.New(cfg, "test")
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

	objStore := objectstorage.NewStore(t.TempDir(), log)
	if err := objStore.InitRoot(); err != nil {
		pool.Close()
		t.Fatalf("init object storage: %v", err)
	}

	cleanup := func() { pool.Close() }
	return pool, postgres.NewObjectStore(pool, log), objStore, postgres.NewIdempotencyStore(pool, log), log, cleanup
}

func TestObjectFunctions_GetObjectManifestReadsFilesystem(t *testing.T) {
	_, pgStore, objStore, idemStore, log, cleanup := testFunctionStores(t)
	defer cleanup()

	f := NewObjectFunctions(pgStore, objStore, idemStore, log)
	ctx := context.Background()
	obj := &model.Object{
		ObjectID:  "manifest_obj",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := f.CreateObject(ctx, obj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

	dbManifest := &model.ObjectManifest{
		Files: map[string]model.ObjectFileInfo{
			"db.txt": {Size: 1, UpdatedAt: mustParseTime(t, "2026-05-03T00:00:00Z")},
		},
	}
	if err := pgStore.UpdateObjectManifest(ctx, obj.ObjectID, dbManifest); err != nil {
		t.Fatalf("UpdateObjectManifest cache failed: %v", err)
	}

	fsManifest := &model.ObjectManifest{
		Files: map[string]model.ObjectFileInfo{
			"fs.txt": {Size: 2, UpdatedAt: mustParseTime(t, "2026-05-03T01:00:00Z")},
		},
	}
	data, err := json.Marshal(model.NormalizeManifest(fsManifest))
	if err != nil {
		t.Fatalf("Marshal filesystem manifest failed: %v", err)
	}
	if err := objStore.WriteManifestFile(obj.ObjectID, data); err != nil {
		t.Fatalf("WriteManifestFile failed: %v", err)
	}

	got, err := f.GetObjectManifest(ctx, obj.ObjectID)
	if err != nil {
		t.Fatalf("GetObjectManifest failed: %v", err)
	}
	if _, ok := got.Files["fs.txt"]; !ok {
		t.Fatalf("expected filesystem manifest, got %+v", got.Files)
	}
	if _, ok := got.Files["db.txt"]; ok {
		t.Fatalf("did not expect stale database manifest, got %+v", got.Files)
	}
}

func TestObjectFunctions_UpdateObjectManifestSyncsFilesystemAndDBCache(t *testing.T) {
	_, pgStore, objStore, idemStore, log, cleanup := testFunctionStores(t)
	defer cleanup()

	f := NewObjectFunctions(pgStore, objStore, idemStore, log)
	ctx := context.Background()
	obj := &model.Object{
		ObjectID:  "manifest_sync_obj",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := f.CreateObject(ctx, obj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

	manifest := &model.ObjectManifest{
		Files: map[string]model.ObjectFileInfo{
			"data.txt": {Size: 4, UpdatedAt: mustParseTime(t, "2026-05-03T02:00:00Z")},
		},
	}
	if err := f.UpdateObjectManifest(ctx, obj.ObjectID, manifest); err != nil {
		t.Fatalf("UpdateObjectManifest failed: %v", err)
	}

	fsData, err := objStore.ReadManifestFile(obj.ObjectID)
	if err != nil {
		t.Fatalf("ReadManifestFile failed: %v", err)
	}
	var fsManifest model.ObjectManifest
	if err := json.Unmarshal(fsData, &fsManifest); err != nil {
		t.Fatalf("Unmarshal filesystem manifest failed: %v", err)
	}
	if fsManifest.Files["data.txt"].Size != 4 {
		t.Fatalf("expected filesystem manifest size 4, got %+v", fsManifest.Files)
	}

	dbManifest, err := pgStore.GetObjectManifest(ctx, obj.ObjectID)
	if err != nil {
		t.Fatalf("GetObjectManifest cache failed: %v", err)
	}
	if dbManifest.Files["data.txt"].Size != 4 {
		t.Fatalf("expected database cache manifest size 4, got %+v", dbManifest.Files)
	}
	if dbManifest.Version == "" {
		t.Fatal("expected manifest version to be populated")
	}
}

func TestObjectFunctions_CreateObjectIdempotencyKey(t *testing.T) {
	pool, pgStore, objStore, idemStore, log, cleanup := testFunctionStores(t)
	defer cleanup()

	f := NewObjectFunctions(pgStore, objStore, idemStore, log)
	ctx := context.Background()

	make := func(id string) *model.Object {
		return &model.Object{
			ObjectID:  id,
			Type:      model.ObjectTypeLog,
			OwnerType: model.OwnerTypeSystem,
			OwnerID:   "system",
			JSON:      []byte(`{}`),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
	}

	// First call with a new key creates the object.
	if err := f.CreateObject(ctx, make("idem_obj_a"), WithIdempotencyKey("client-1")); err != nil {
		t.Fatalf("first CreateObject failed: %v", err)
	}

	// Replay with the same key + same object_id is a successful no-op.
	if err := f.CreateObject(ctx, make("idem_obj_a"), WithIdempotencyKey("client-1")); err != nil {
		t.Fatalf("idempotent retry failed: %v", err)
	}

	// Same key with a different object_id is a conflict.
	err := f.CreateObject(ctx, make("idem_obj_b"), WithIdempotencyKey("client-1"))
	if err == nil {
		t.Fatal("expected conflict for key reuse with different object_id")
	}
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) || fieldErr.Code != "CONFLICT" {
		t.Fatalf("expected CONFLICT field error, got %T: %v", err, err)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM idempotency_keys WHERE scope = 'object_create' AND key = $1`,
		"client-1",
	).Scan(&status); err != nil {
		t.Fatalf("read idempotency status: %v", err)
	}
	if status != "completed" {
		t.Fatalf("expected completed idempotency status, got %q", status)
	}
}

func TestObjectFunctions_CreateObjectRecoversPendingIdempotencyKey(t *testing.T) {
	pool, pgStore, objStore, idemStore, log, cleanup := testFunctionStores(t)
	defer cleanup()

	f := NewObjectFunctions(pgStore, objStore, idemStore, log)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO idempotency_keys (scope, key, resource_id, status) VALUES ('object_create', 'client-pending', 'recovered_obj', 'pending')`,
	); err != nil {
		t.Fatalf("seed pending idempotency key: %v", err)
	}

	obj := &model.Object{
		ObjectID:  "recovered_obj",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
	}
	if err := f.CreateObject(ctx, obj, WithIdempotencyKey("client-pending")); err != nil {
		t.Fatalf("CreateObject recovery failed: %v", err)
	}
	if _, err := pgStore.GetObject(ctx, obj.ObjectID); err != nil {
		t.Fatalf("expected recovered object row: %v", err)
	}
	if _, err := objStore.ReadManifestFile(obj.ObjectID); err != nil {
		t.Fatalf("expected recovered manifest: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM idempotency_keys WHERE scope = 'object_create' AND key = $1`,
		"client-pending",
	).Scan(&status); err != nil {
		t.Fatalf("read recovered idempotency status: %v", err)
	}
	if status != "completed" {
		t.Fatalf("expected recovered object status completed, got %q", status)
	}
}

func TestTaskFunctions_CreateTaskRecoversPendingIdempotencyKey(t *testing.T) {
	pool, pgStore, objStore, idemStore, log, cleanup := testFunctionStores(t)
	defer cleanup()

	objectFuncs := NewObjectFunctions(pgStore, objStore, idemStore, log)
	ctx := context.Background()
	commandCatalog := &model.Object{
		ObjectID:  "command_catalog",
		Type:      model.ObjectTypeDocument,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      validCommandCatalogJSON(),
	}
	if err := objectFuncs.CreateObject(ctx, commandCatalog); err != nil {
		t.Fatalf("seed command catalog object: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO idempotency_keys (scope, key, resource_id, status) VALUES ('task_create', 'client-pending-task', 'recovered_task', 'pending')`,
	); err != nil {
		t.Fatalf("seed pending task idempotency key: %v", err)
	}

	taskFuncs := NewTaskFunctions(postgres.NewTaskStore(pool, log), postgres.NewEntityStore(pool, log), pgStore, idemStore, log)
	task := &model.Task{
		TaskID:                 "recovered_task",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   []byte(`{"components":{"command":{"type":"move_to_location"},"parameters":{}}}`),
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO entities (entity_id, type, json, version, created_at, updated_at)
		 VALUES ('asset_001', 'asset', '{"components":{"supported_commands":{"commands":["move_to_location"]}}}'::jsonb, 1, NOW(), NOW())`,
	); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	if err := taskFuncs.CreateTask(ctx, task, WithIdempotencyKey("client-pending-task")); err != nil {
		t.Fatalf("CreateTask recovery failed: %v", err)
	}

	if _, err := postgres.NewTaskStore(pool, log).GetTask(ctx, task.TaskID); err != nil {
		t.Fatalf("expected recovered task row: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM idempotency_keys WHERE scope = 'task_create' AND key = $1`,
		"client-pending-task",
	).Scan(&status); err != nil {
		t.Fatalf("read recovered task idempotency status: %v", err)
	}
	if status != "completed" {
		t.Fatalf("expected recovered task status completed, got %q", status)
	}
}
