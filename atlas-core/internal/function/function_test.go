package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/config"
	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/store"
)

func testLogger() *logging.Logger {
	return logging.New(&config.Config{LogLevel: "debug"}, "test")
}

type fakeEntityStore struct{}

func (fakeEntityStore) CreateEntity(context.Context, *model.Entity) error        { return nil }
func (fakeEntityStore) GetEntity(context.Context, string) (*model.Entity, error) { return nil, nil }
func (fakeEntityStore) ListEntities(context.Context, ...store.EntityFilter) ([]model.Entity, error) {
	return nil, nil
}
func (fakeEntityStore) UpdateEntity(context.Context, *model.Entity) error { return nil }
func (fakeEntityStore) DeleteEntity(context.Context, string) error        { return nil }
func (fakeEntityStore) UpsertEntity(context.Context, *model.Entity) error { return nil }

type fakeTaskStore struct {
	createFn func(context.Context, *model.Task) error
	getFn    func(context.Context, string) (*model.Task, error)
	updateFn func(context.Context, *model.Task) error
	deleteFn func(context.Context, string) error
	upsertFn func(context.Context, *model.Task) error
}

func (s fakeTaskStore) CreateTask(ctx context.Context, task *model.Task) error {
	if s.createFn != nil {
		return s.createFn(ctx, task)
	}
	return nil
}
func (s fakeTaskStore) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	if s.getFn != nil {
		return s.getFn(ctx, taskID)
	}
	return nil, model.ErrNotFound
}
func (s fakeTaskStore) ListTasks(context.Context, ...store.TaskFilter) ([]model.Task, error) {
	return nil, nil
}
func (s fakeTaskStore) UpdateTask(ctx context.Context, task *model.Task) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, task)
	}
	return nil
}
func (s fakeTaskStore) DeleteTask(ctx context.Context, taskID string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, taskID)
	}
	return nil
}
func (s fakeTaskStore) UpsertTask(ctx context.Context, task *model.Task) error {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, task)
	}
	return nil
}

type fakeObservationStore struct{}

func (fakeObservationStore) CreateObservation(context.Context, *model.Observation) error { return nil }
func (fakeObservationStore) GetObservation(context.Context, string) (*model.Observation, error) {
	return nil, nil
}
func (fakeObservationStore) ListObservations(context.Context, ...store.ObservationFilter) ([]model.Observation, error) {
	return nil, nil
}
func (fakeObservationStore) UpdateObservation(context.Context, *model.Observation) error { return nil }
func (fakeObservationStore) DeleteObservation(context.Context, string) error             { return nil }
func (fakeObservationStore) UpsertObservation(context.Context, *model.Observation) error { return nil }

type fakeObjectStore struct {
	createFn             func(context.Context, *model.Object) error
	getFn                func(context.Context, string) (*model.Object, error)
	listFn               func(context.Context, ...store.ObjectFilter) ([]model.Object, error)
	updateFn             func(context.Context, *model.Object) error
	deleteFn             func(context.Context, string) error
	upsertFn             func(context.Context, *model.Object) error
	updateManifestFn     func(context.Context, string, *model.ObjectManifest, ...time.Time) error
	getManifestFn        func(context.Context, string) (*model.ObjectManifest, error)
	updatedManifestCalls int
}

func (s *fakeObjectStore) CreateObject(ctx context.Context, obj *model.Object) error {
	if s.createFn != nil {
		return s.createFn(ctx, obj)
	}
	return nil
}
func (s *fakeObjectStore) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	if s.getFn != nil {
		return s.getFn(ctx, objectID)
	}
	return nil, model.ErrNotFound
}
func (s *fakeObjectStore) ListObjects(ctx context.Context, filters ...store.ObjectFilter) ([]model.Object, error) {
	if s.listFn != nil {
		return s.listFn(ctx, filters...)
	}
	return nil, nil
}
func (s *fakeObjectStore) UpdateObject(ctx context.Context, obj *model.Object) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, obj)
	}
	return nil
}
func (s *fakeObjectStore) DeleteObject(ctx context.Context, objectID string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, objectID)
	}
	return nil
}
func (s *fakeObjectStore) UpsertObject(ctx context.Context, obj *model.Object) error {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, obj)
	}
	return nil
}
func (s *fakeObjectStore) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest, updatedAt ...time.Time) error {
	s.updatedManifestCalls++
	if s.updateManifestFn != nil {
		return s.updateManifestFn(ctx, objectID, manifest, updatedAt...)
	}
	return nil
}
func (s *fakeObjectStore) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	if s.getManifestFn != nil {
		return s.getManifestFn(ctx, objectID)
	}
	return model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}), nil
}

type fakeObjectStorage struct {
	createFolderFn  func(string) error
	existsFn        func(string) (bool, error)
	listFoldersFn   func() ([]string, error)
	deleteFolderFn  func(string) error
	readFileFn      func(string, string) ([]byte, error)
	writeFileFn     func(string, string, []byte) error
	appendFileFn    func(string, string, []byte) error
	deleteFileFn    func(string, string) error
	listFilesFn     func(string) ([]string, error)
	readManifestFn  func(string) ([]byte, error)
	writeManifestFn func(string, []byte) error
	validatePathFn  func(string, string) error
}

func (s fakeObjectStorage) CreateObjectFolder(objectID string) error {
	if s.createFolderFn != nil {
		return s.createFolderFn(objectID)
	}
	return nil
}
func (s fakeObjectStorage) ObjectFolderExists(objectID string) (bool, error) {
	if s.existsFn != nil {
		return s.existsFn(objectID)
	}
	return false, nil
}
func (s fakeObjectStorage) ListObjectFolders() ([]string, error) {
	if s.listFoldersFn != nil {
		return s.listFoldersFn()
	}
	return nil, nil
}
func (s fakeObjectStorage) DeleteObjectFolder(objectID string) error {
	if s.deleteFolderFn != nil {
		return s.deleteFolderFn(objectID)
	}
	return nil
}
func (s fakeObjectStorage) WriteObjectFile(objectID, filename string, data []byte) error {
	if s.writeFileFn != nil {
		return s.writeFileFn(objectID, filename, data)
	}
	return nil
}
func (s fakeObjectStorage) AppendObjectFile(objectID, filename string, data []byte) error {
	if s.appendFileFn != nil {
		return s.appendFileFn(objectID, filename, data)
	}
	return nil
}
func (s fakeObjectStorage) ReadObjectFile(objectID, filename string) ([]byte, error) {
	if s.readFileFn != nil {
		return s.readFileFn(objectID, filename)
	}
	return nil, nil
}
func (s fakeObjectStorage) DeleteObjectFile(objectID, filename string) error {
	if s.deleteFileFn != nil {
		return s.deleteFileFn(objectID, filename)
	}
	return nil
}
func (s fakeObjectStorage) ListObjectFolderFiles(objectID string) ([]string, error) {
	if s.listFilesFn != nil {
		return s.listFilesFn(objectID)
	}
	return nil, nil
}
func (s fakeObjectStorage) ReadManifestFile(objectID string) ([]byte, error) {
	if s.readManifestFn != nil {
		return s.readManifestFn(objectID)
	}
	return json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}))
}
func (s fakeObjectStorage) WriteManifestFile(objectID string, data []byte) error {
	if s.writeManifestFn != nil {
		return s.writeManifestFn(objectID, data)
	}
	return nil
}
func (s fakeObjectStorage) ValidateSafeObjectPath(objectID, filename string) error {
	if s.validatePathFn != nil {
		return s.validatePathFn(objectID, filename)
	}
	return nil
}
func (s fakeObjectStorage) ReaderForObjectFile(string, string) (io.ReadCloser, error) {
	return nil, nil
}

type fakeIdempotencyStore struct {
	tryBeginFn      func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error)
	markCompletedFn func(context.Context, string, string) error
	markFailedFn    func(context.Context, string, string) error
}

func (s fakeIdempotencyStore) TryBegin(ctx context.Context, scope, key, resourceID string) (store.IdempotencyRecord, bool, error) {
	if s.tryBeginFn != nil {
		return s.tryBeginFn(ctx, scope, key, resourceID)
	}
	return store.IdempotencyRecord{ResourceID: resourceID, Status: store.IdempotencyStatusPending}, true, nil
}

func (s fakeIdempotencyStore) MarkCompleted(ctx context.Context, scope, key string) error {
	if s.markCompletedFn != nil {
		return s.markCompletedFn(ctx, scope, key)
	}
	return nil
}

func (s fakeIdempotencyStore) MarkFailed(ctx context.Context, scope, key string) error {
	if s.markFailedFn != nil {
		return s.markFailedFn(ctx, scope, key)
	}
	return nil
}

func TestEntityFunctions_ValidateEntityID(t *testing.T) {
	f := EntityFunctions{}
	entity := &model.Entity{Type: model.EntityTypeAsset, JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateEntity(nil, entity); err == nil {
		t.Fatal("expected error for empty entity_id")
	}
	entity.EntityID = "this-entity-id-is-way-too-long-for-the-50-character-limit"
	if err := f.CreateEntity(nil, entity); err == nil {
		t.Fatal("expected error for long entity_id")
	}
}

func TestEntityFunctions_ValidateType(t *testing.T) {
	f := EntityFunctions{}
	entity := &model.Entity{EntityID: "test_001", Type: model.EntityType("invalid_type"), JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateEntity(nil, entity); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestObservationFunctions_ValidateObservationID(t *testing.T) {
	f := ObservationFunctions{}
	obs := &model.Observation{SourceAssetID: "asset_001", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObservation(nil, obs); err == nil {
		t.Fatal("expected error for empty observation_id")
	}
}

func TestObservationFunctions_ValidateSourceAssetID(t *testing.T) {
	f := ObservationFunctions{}
	obs := &model.Observation{ObservationID: "obs_001", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObservation(nil, obs); err == nil {
		t.Fatal("expected error for empty source_asset_id")
	}
}

func TestTaskFunctions_ValidateRequiredFields(t *testing.T) {
	f := TaskFunctions{}
	task := &model.Task{TaskID: "task_001", AssetID: "asset_001", CommandCatalogObjectID: "cmd_001", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateTask(nil, task); err == nil {
		t.Fatal("expected error for empty status")
	}
	task.Status = model.TaskStatusPending
	task.AssetID = ""
	if err := f.CreateTask(nil, task); err == nil {
		t.Fatal("expected error for empty asset_id")
	}
	task.AssetID = "asset_001"
	task.CommandCatalogObjectID = ""
	if err := f.CreateTask(nil, task); err == nil {
		t.Fatal("expected error for empty command_catalog_object_id")
	}
}

func TestTaskFunctions_RejectsNonCommandCatalogObject(t *testing.T) {
	f := NewTaskFunctions(fakeTaskStore{}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog}, nil
	}}, fakeIdempotencyStore{}, testLogger())
	task := &model.Task{TaskID: "task_001", Status: model.TaskStatusPending, AssetID: "asset_001", CommandCatalogObjectID: "obj_001"}
	if err := f.CreateTask(context.Background(), task); err == nil {
		t.Fatal("expected task validation failure")
	}
}

func TestObjectFunctions_ValidateRequiredFields(t *testing.T) {
	f := ObjectFunctions{}
	obj := &model.Object{ObjectID: "obj_001", OwnerType: model.OwnerTypeSystem, OwnerID: "system", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObject(nil, obj); err == nil {
		t.Fatal("expected error for empty type")
	}
	obj.Type = model.ObjectTypeLog
	obj.OwnerType = ""
	if err := f.CreateObject(nil, obj); err == nil {
		t.Fatal("expected error for empty owner_type")
	}
	obj.OwnerType = model.OwnerTypeSystem
	obj.OwnerID = ""
	if err := f.CreateObject(nil, obj); err == nil {
		t.Fatal("expected error for empty owner_id")
	}
}

func TestFunctions_RejectNilModels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "entity", err: EntityFunctions{}.CreateEntity(context.Background(), nil)},
		{name: "object", err: ObjectFunctions{}.CreateObject(context.Background(), nil)},
		{name: "task", err: TaskFunctions{}.CreateTask(context.Background(), nil)},
		{name: "observation", err: ObservationFunctions{}.CreateObservation(context.Background(), nil)},
	}
	for _, tt := range tests {
		if tt.err == nil {
			t.Fatalf("expected error for nil %s model", tt.name)
		}
	}
}

func TestObjectFunctions_RejectUnsafeObjectID(t *testing.T) {
	f := ObjectFunctions{}
	obj := &model.Object{ObjectID: "../obj", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObject(context.Background(), obj); err == nil {
		t.Fatal("expected error for unsafe object_id")
	}
}

func TestObjectFunctions_UpdateObjectManifestRejectsNil(t *testing.T) {
	f := ObjectFunctions{}
	if err := f.UpdateObjectManifest(context.Background(), "obj_001", nil); err == nil {
		t.Fatal("expected error for nil manifest")
	}
}

func TestObjectFunctions_CreateObjectRollsBackMetadataOnStorageFailure(t *testing.T) {
	deleted := false
	pg := &fakeObjectStore{
		createFn: func(context.Context, *model.Object) error { return nil },
		deleteFn: func(context.Context, string) error { deleted = true; return nil },
	}
	storage := fakeObjectStorage{createFolderFn: func(string) error { return fmt.Errorf("boom") }}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	obj := &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}
	if err := f.CreateObject(context.Background(), obj); err == nil {
		t.Fatal("expected create object failure")
	}
	if !deleted {
		t.Fatal("expected metadata rollback to run")
	}
}

func TestObjectFunctions_CreateObjectReportsRollbackFailure(t *testing.T) {
	pg := &fakeObjectStore{
		createFn: func(context.Context, *model.Object) error { return nil },
		deleteFn: func(context.Context, string) error { return fmt.Errorf("delete failed") },
	}
	storage := fakeObjectStorage{
		createFolderFn: func(string) error { return fmt.Errorf("boom") },
		deleteFolderFn: func(string) error { return fmt.Errorf("cleanup failed") },
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	obj := &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}
	err := f.CreateObject(context.Background(), obj)
	if err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("expected rollback failure details, got %v", err)
	}
}

func TestObjectFunctions_CreateObjectDoesNotFailOnManifestCacheRefreshFailure(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}))
	deleted := false
	pg := &fakeObjectStore{
		createFn:      func(context.Context, *model.Object) error { return nil },
		deleteFn:      func(context.Context, string) error { deleted = true; return nil },
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) { return nil, model.ErrNotFound },
		updateManifestFn: func(context.Context, string, *model.ObjectManifest, ...time.Time) error {
			return fmt.Errorf("cache unavailable")
		},
	}
	storage := fakeObjectStorage{
		createFolderFn: func(string) error { return nil },
		readManifestFn: func(string) ([]byte, error) { return manifestData, nil },
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())

	if err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}); err != nil {
		t.Fatalf("expected create to succeed despite manifest cache refresh failure, got %v", err)
	}
	if deleted {
		t.Fatal("did not expect rollback after durable object creation")
	}
}

func TestObjectFunctions_DeleteObjectRestoresMetadataOnStorageFailure(t *testing.T) {
	upserted := false
	pg := &fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}, nil
		},
		deleteFn: func(context.Context, string) error { return nil },
		upsertFn: func(context.Context, *model.Object) error { upserted = true; return nil },
	}
	storage := fakeObjectStorage{deleteFolderFn: func(string) error { return fmt.Errorf("storage delete failed") }}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	if err := f.DeleteObject(context.Background(), "obj_001"); err == nil {
		t.Fatal("expected delete failure")
	}
	if !upserted {
		t.Fatal("expected metadata restore on storage failure")
	}
}

func TestObjectFunctions_UpsertObjectDoesNotFailOnManifestCacheRefreshFailure(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}))
	deleted := false
	pg := &fakeObjectStore{
		getFn:         func(context.Context, string) (*model.Object, error) { return nil, model.ErrNotFound },
		upsertFn:      func(context.Context, *model.Object) error { return nil },
		deleteFn:      func(context.Context, string) error { deleted = true; return nil },
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) { return nil, model.ErrNotFound },
		updateManifestFn: func(context.Context, string, *model.ObjectManifest, ...time.Time) error {
			return fmt.Errorf("cache unavailable")
		},
	}
	storage := fakeObjectStorage{
		existsFn:       func(string) (bool, error) { return false, nil },
		createFolderFn: func(string) error { return nil },
		readManifestFn: func(string) ([]byte, error) { return manifestData, nil },
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())

	if err := f.UpsertObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}); err != nil {
		t.Fatalf("expected upsert to succeed despite manifest cache refresh failure, got %v", err)
	}
	if deleted {
		t.Fatal("did not expect rollback after durable object upsert")
	}
}

func TestObjectFunctions_CreateObjectRecoversPendingIdempotencyClaim(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}))
	created := false
	completed := false
	pg := &fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			if created {
				return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}, nil
			}
			return nil, model.ErrNotFound
		},
		createFn: func(context.Context, *model.Object) error {
			created = true
			return nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return nil, model.ErrNotFound
		},
	}
	storage := fakeObjectStorage{
		createFolderFn: func(string) error { return nil },
		readManifestFn: func(string) ([]byte, error) { return manifestData, nil },
	}
	idem := fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_001", Status: store.IdempotencyStatusPending}, false, nil
		},
		markCompletedFn: func(context.Context, string, string) error {
			completed = true
			return nil
		},
	}
	f := NewObjectFunctions(pg, storage, idem, testLogger())

	if err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("client-1")); err != nil {
		t.Fatalf("expected pending claim recovery to succeed, got %v", err)
	}
	if !created {
		t.Fatal("expected object creation to resume for pending claim")
	}
	if !completed {
		t.Fatal("expected idempotency key to be marked completed")
	}
}

func TestTaskFunctions_CreateTaskRecoversPendingIdempotencyClaim(t *testing.T) {
	created := false
	completed := false
	taskStore := fakeTaskStore{
		createFn: func(context.Context, *model.Task) error {
			created = true
			return nil
		},
		getFn: func(context.Context, string) (*model.Task, error) {
			if created {
				return &model.Task{TaskID: "task_001", Status: model.TaskStatusPending, AssetID: "asset_001", CommandCatalogObjectID: "cmd_001"}, nil
			}
			return nil, model.ErrNotFound
		},
	}
	objectStore := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog}, nil
	}}
	idem := fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "task_001", Status: store.IdempotencyStatusPending}, false, nil
		},
		markCompletedFn: func(context.Context, string, string) error {
			completed = true
			return nil
		},
	}
	f := NewTaskFunctions(taskStore, objectStore, idem, testLogger())

	if err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
	}, WithIdempotencyKey("client-1")); err != nil {
		t.Fatalf("expected pending task claim recovery to succeed, got %v", err)
	}
	if !created {
		t.Fatal("expected task creation to resume for pending claim")
	}
	if !completed {
		t.Fatal("expected task idempotency key to be marked completed")
	}
}

func TestObjectFunctions_ReconcileRepairsDrift(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{"a.txt": {Size: 1, UpdatedAt: time.Now().UTC()}}}))
	deletedFolder := ""
	createdFolder := ""
	pg := &fakeObjectStore{
		listFn: func(context.Context, ...store.ObjectFilter) ([]model.Object, error) {
			return []model.Object{{ObjectID: "db_only", Type: model.ObjectTypeLog}, {ObjectID: "shared", Type: model.ObjectTypeLog}}, nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}), nil
		},
	}
	storage := fakeObjectStorage{
		listFoldersFn:  func() ([]string, error) { return []string{"shared", "fs_only"}, nil },
		deleteFolderFn: func(objectID string) error { deletedFolder = objectID; return nil },
		createFolderFn: func(objectID string) error { createdFolder = objectID; return nil },
		readManifestFn: func(string) ([]byte, error) { return manifestData, nil },
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	if err := f.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if deletedFolder != "fs_only" {
		t.Fatalf("expected fs_only folder deletion, got %q", deletedFolder)
	}
	if createdFolder != "db_only" {
		t.Fatalf("expected db_only folder recreation, got %q", createdFolder)
	}
	if pg.updatedManifestCalls == 0 {
		t.Fatal("expected manifest cache sync during reconciliation")
	}
}

func TestObjectFunctions_ReconcileRepairsMissingManifest(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}))
	repaired := false
	pg := &fakeObjectStore{
		listFn: func(context.Context, ...store.ObjectFilter) ([]model.Object, error) {
			return []model.Object{{ObjectID: "shared", Type: model.ObjectTypeLog}}, nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return nil, model.ErrNotFound
		},
	}
	storage := fakeObjectStorage{
		listFoldersFn: func() ([]string, error) { return []string{"shared"}, nil },
		readManifestFn: func(string) ([]byte, error) {
			if !repaired {
				return nil, model.ErrNotFound
			}
			return manifestData, nil
		},
		writeManifestFn: func(string, []byte) error {
			repaired = true
			return nil
		},
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	if err := f.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !repaired {
		t.Fatal("expected reconcile to repair missing manifest")
	}
}

func TestModelErrors_IsCoreError(t *testing.T) {
	wrapped := fmt.Errorf("wrapped: %w", model.ErrNotFound)
	if !errors.Is(wrapped, model.ErrNotFound) {
		t.Fatal("expected wrapped core error to match with errors.Is")
	}
}
