package function

import (
	"atlas.local/protocol"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

func testLogger() *logging.Logger {
	return logging.New("debug", "atlas-test", "test")
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed.UTC()
}

func testProtoValidator() *protocolvalidation.Validator {
	v, err := protocolvalidation.New()
	if err != nil {
		panic(fmt.Sprintf("init protocol validator: %v", err))
	}
	return v
}

type fakeProtocolValidator struct {
	entityIssues             []protocol.ValidationIssue
	objectIssues             []protocol.ValidationIssue
	taskIssues               []protocol.ValidationIssue
	observationIssues        []protocol.ValidationIssue
	commandCatalogJSONIssues []protocol.ValidationIssue
}

func (f fakeProtocolValidator) ValidateEntity(entity *model.Entity) []protocol.ValidationIssue {
	return f.entityIssues
}

func (f fakeProtocolValidator) ValidateObject(obj *model.Object) []protocol.ValidationIssue {
	return f.objectIssues
}

func (f fakeProtocolValidator) ValidateTask(task *model.Task) []protocol.ValidationIssue {
	return f.taskIssues
}

func (f fakeProtocolValidator) ValidateObservation(obs *model.Observation) []protocol.ValidationIssue {
	return f.observationIssues
}

func (f fakeProtocolValidator) ValidateCommandCatalogJSON(data []byte) []protocol.ValidationIssue {
	return f.commandCatalogJSONIssues
}

type fakeEntityStore struct {
	getFn func(context.Context, string) (*model.Entity, error)
}

func (s *fakeEntityStore) CreateEntity(context.Context, *model.Entity) error { return nil }
func (s *fakeEntityStore) GetEntity(ctx context.Context, id string) (*model.Entity, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return nil, model.ErrNotFound
}
func (s *fakeEntityStore) ListEntities(context.Context, ...store.EntityFilter) ([]model.Entity, error) {
	return nil, nil
}
func (s *fakeEntityStore) UpdateEntity(context.Context, *model.Entity) error { return nil }
func (s *fakeEntityStore) DeleteEntity(context.Context, string) error        { return nil }
func (s *fakeEntityStore) UpsertEntity(context.Context, *model.Entity) error { return nil }

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
	fileInfoFn      func(string, string) (model.ObjectFileInfo, error)
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
func (s fakeObjectStorage) GetObjectFileInfo(objectID, filename string) (model.ObjectFileInfo, error) {
	if s.fileInfoFn != nil {
		return s.fileInfoFn(objectID, filename)
	}
	return model.ObjectFileInfo{}, nil
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

type capturePublisher struct {
	events []*sharedv1.MutationEvent
}

func (p *capturePublisher) Publish(_ context.Context, event *sharedv1.MutationEvent) {
	p.events = append(p.events, event)
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
	}}, &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`)}, nil
	}}, fakeIdempotencyStore{}, testLogger(), testProtoValidator())
	task := &model.Task{TaskID: "task_001", Status: model.TaskStatusPending, AssetID: "asset_001", CommandCatalogObjectID: "obj_001", JSON: []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{}}}`)}
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
	ctx := context.Background()
	tests := []struct {
		name string
		err  error
	}{
		{name: "entity CreateEntity", err: EntityFunctions{}.CreateEntity(ctx, nil)},
		{name: "entity UpdateEntity", err: EntityFunctions{}.UpdateEntity(ctx, nil)},
		{name: "entity UpsertEntity", err: EntityFunctions{}.UpsertEntity(ctx, nil)},
		{name: "object CreateObject", err: ObjectFunctions{}.CreateObject(ctx, nil)},
		{name: "object UpdateObject", err: ObjectFunctions{}.UpdateObject(ctx, nil)},
		{name: "object UpsertObject", err: ObjectFunctions{}.UpsertObject(ctx, nil)},
		{name: "object UpdateObjectManifest", err: ObjectFunctions{}.UpdateObjectManifest(ctx, "obj_001", nil)},
		{name: "task CreateTask", err: TaskFunctions{}.CreateTask(ctx, nil)},
		{name: "task UpdateTask", err: TaskFunctions{}.UpdateTask(ctx, nil)},
		{name: "task UpsertTask", err: TaskFunctions{}.UpsertTask(ctx, nil)},
		{name: "observation CreateObservation", err: ObservationFunctions{}.CreateObservation(ctx, nil)},
		{name: "observation UpdateObservation", err: ObservationFunctions{}.UpdateObservation(ctx, nil)},
		{name: "observation UpsertObservation", err: ObservationFunctions{}.UpsertObservation(ctx, nil)},
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

func TestLocalObjectGateway_SyncObjectManifestFromFilesystemIgnoringErrorsIgnoresUnexpectedError(t *testing.T) {
	gateway := &localObjectGateway{
		metadata: &fakeObjectStore{
			updateManifestFn: func(context.Context, string, *model.ObjectManifest, ...time.Time) error {
				return errors.New("cache refresh failed")
			},
		},
		files: fakeObjectStorage{
			readManifestFn: func(string) ([]byte, error) {
				return []byte(`{"files":{}}`), nil
			},
		},
	}

	if err := gateway.syncObjectManifestFromFilesystemIgnoringErrors(context.Background(), "obj_001"); err != nil {
		t.Fatalf("expected cache refresh error to be ignored, got %v", err)
	}
}

func TestLocalObjectGateway_SyncObjectManifestFromFilesystemIgnoringErrorsIgnoresMissingManifest(t *testing.T) {
	gateway := &localObjectGateway{
		metadata: &fakeObjectStore{},
		files: fakeObjectStorage{
			readManifestFn: func(string) ([]byte, error) {
				return nil, model.ErrNotFound
			},
		},
	}

	if err := gateway.syncObjectManifestFromFilesystemIgnoringErrors(context.Background(), "obj_001"); err != nil {
		t.Fatalf("expected missing manifest to be ignored, got %v", err)
	}
}

func TestObjectFunctions_CreateObjectRollsBackMetadataOnStorageFailure(t *testing.T) {
	deleted := false
	pg := &fakeObjectStore{
		createFn: func(context.Context, *model.Object) error { return nil },
		deleteFn: func(context.Context, string) error { deleted = true; return nil },
	}
	storage := fakeObjectStorage{createFolderFn: func(string) error { return fmt.Errorf("boom") }}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator())
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
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator())
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
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

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
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator())
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
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

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
	f := NewObjectFunctions(pg, storage, idem, testLogger(), testProtoValidator())

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

func TestObjectFunctions_CreateObjectWithFreshIdempotencyKeyStillConflictsOnDuplicateID(t *testing.T) {
	markedFailed := false
	pg := &fakeObjectStore{
		createFn: func(context.Context, *model.Object) error { return model.ErrConflict },
	}
	f := NewObjectFunctions(pg, fakeObjectStorage{}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger(), testProtoValidator())

	err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("fresh-key"))
	if !errors.Is(err, model.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if !markedFailed {
		t.Fatal("expected fresh idempotency claim to be marked failed on duplicate object")
	}
}

func TestObjectFunctions_CreateObjectMarksClaimFailedOnValidationError(t *testing.T) {
	markedFailed := false
	f := NewObjectFunctions(&fakeObjectStore{}, fakeObjectStorage{}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger(), fakeProtocolValidator{
		objectIssues: []protocol.ValidationIssue{{Field: "json", Code: "invalid_json", Message: "invalid"}},
	})

	err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("fresh-key"))
	if err == nil {
		t.Fatal("expected validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if !markedFailed {
		t.Fatal("expected fresh object idempotency claim to be marked failed on validation error")
	}
}

func TestObjectFunctions_CreateObjectJoinsMarkFailedErrorOnValidationFailure(t *testing.T) {
	markErr := errors.New("mark failed")
	f := NewObjectFunctions(&fakeObjectStore{}, fakeObjectStorage{}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			return markErr
		},
	}, testLogger(), fakeProtocolValidator{
		objectIssues: []protocol.ValidationIssue{{Field: "json", Code: "invalid_json", Message: "invalid"}},
	})

	err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("fresh-key"))
	if err == nil {
		t.Fatal("expected joined error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError in joined error, got %T: %v", err, err)
	}
	if !errors.Is(err, markErr) {
		t.Fatalf("expected joined error to include mark failure, got %v", err)
	}
}

func TestObjectFunctions_ReadFileRequiresObjectRow(t *testing.T) {
	readCalled := false
	f := NewObjectFunctions(&fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			return nil, model.ErrNotFound
		},
	}, fakeObjectStorage{
		readFileFn: func(string, string) ([]byte, error) {
			readCalled = true
			return []byte("secret"), nil
		},
	}, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	_, err := f.ReadFile(context.Background(), "obj_001", "data.txt")
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if readCalled {
		t.Fatal("expected missing object row to prevent file read")
	}
}

func TestObjectFunctions_ListFilesRequiresObjectRow(t *testing.T) {
	listCalled := false
	f := NewObjectFunctions(&fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			return nil, model.ErrNotFound
		},
	}, fakeObjectStorage{
		listFilesFn: func(string) ([]string, error) {
			listCalled = true
			return []string{"data.txt"}, nil
		},
	}, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	_, err := f.ListFiles(context.Background(), "obj_001")
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if listCalled {
		t.Fatal("expected missing object row to prevent file listing")
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
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{}}]}`)}, nil
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
	f := NewTaskFunctions(taskStore, objectStore, &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`)}, nil
	}}, idem, testLogger(), testProtoValidator())

	if err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{}}}`),
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

func TestTaskFunctions_CreateTaskWithFreshIdempotencyKeyStillConflictsOnDuplicateID(t *testing.T) {
	markedFailed := false
	taskStore := fakeTaskStore{
		createFn: func(context.Context, *model.Task) error { return model.ErrConflict },
	}
	objectStore := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{}}]}`)}, nil
	}}
	f := NewTaskFunctions(taskStore, objectStore, &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`)}, nil
	}}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "task_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger(), testProtoValidator())

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{}}}`),
	}, WithIdempotencyKey("fresh-key"))
	if !errors.Is(err, model.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if !markedFailed {
		t.Fatal("expected fresh task idempotency claim to be marked failed on duplicate task")
	}
}

func TestTaskFunctions_CreateTaskMarksClaimFailedOnValidationError(t *testing.T) {
	markedFailed := false
	f := NewTaskFunctions(fakeTaskStore{}, &fakeObjectStore{}, &fakeEntityStore{}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "task_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger(), fakeProtocolValidator{
		taskIssues: []protocol.ValidationIssue{{Field: "json", Code: "invalid_json", Message: "invalid"}},
	})

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
	}, WithIdempotencyKey("fresh-key"))
	if err == nil {
		t.Fatal("expected validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if !markedFailed {
		t.Fatal("expected fresh task idempotency claim to be marked failed on validation error")
	}
}

func TestObjectFunctions_ReconcileRepairsDrift(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{"a.txt": {Size: 1, UpdatedAt: time.Now().UTC()}}}))
	deletedFolder := ""
	createdFolder := ""
	restoredObjectID := ""
	pg := &fakeObjectStore{
		createFn: func(_ context.Context, obj *model.Object) error {
			restoredObjectID = obj.ObjectID
			if obj.Type != model.ObjectTypeLog || obj.OwnerType != model.OwnerTypeSystem || obj.OwnerID != "system" {
				t.Fatalf("unexpected restored object metadata: %+v", obj)
			}
			return nil
		},
		listFn: func(context.Context, ...store.ObjectFilter) ([]model.Object, error) {
			return []model.Object{{ObjectID: "db_only", Type: model.ObjectTypeLog}, {ObjectID: "shared", Type: model.ObjectTypeLog}}, nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}), nil
		},
	}
	storage := fakeObjectStorage{
		listFoldersFn:  func() ([]string, error) { return []string{"shared", "fs_only", "no_manifest"}, nil },
		deleteFolderFn: func(objectID string) error { deletedFolder = objectID; return nil },
		createFolderFn: func(objectID string) error { createdFolder = objectID; return nil },
		readManifestFn: func(objectID string) ([]byte, error) {
			if objectID == "no_manifest" {
				return nil, model.ErrNotFound
			}
			return manifestData, nil
		},
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator())
	if err := f.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if restoredObjectID != "fs_only" {
		t.Fatalf("expected fs_only metadata restoration, got %q", restoredObjectID)
	}
	if deletedFolder != "no_manifest" {
		t.Fatalf("expected no_manifest folder deletion, got %q", deletedFolder)
	}
	if createdFolder != "db_only" {
		t.Fatalf("expected db_only folder recreation, got %q", createdFolder)
	}
	if pg.updatedManifestCalls == 0 {
		t.Fatal("expected manifest cache sync during reconciliation")
	}
}

func TestObjectFunctions_ReconcileDeletesInvalidFolders(t *testing.T) {
	deletedFolder := ""
	pg := &fakeObjectStore{
		listFn: func(context.Context, ...store.ObjectFilter) ([]model.Object, error) {
			return []model.Object{{ObjectID: "shared", Type: model.ObjectTypeLog}}, nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}), nil
		},
	}
	storage := fakeObjectStorage{
		listFoldersFn:  func() ([]string, error) { return []string{"../bad", "shared"}, nil },
		deleteFolderFn: func(objectID string) error { deletedFolder = objectID; return nil },
		readManifestFn: func(string) ([]byte, error) { return []byte(`{"files":{}}`), nil },
	}

	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator())
	if err := f.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if deletedFolder != "../bad" {
		t.Fatalf("expected invalid folder deletion, got %q", deletedFolder)
	}
}

func TestObjectFunctions_FileMutationsRebuildAndSyncManifest(t *testing.T) {
	cases := []struct {
		name          string
		mutate        func(ObjectFunctions) error
		expectedFiles map[string]int64
	}{
		{
			name: "write",
			mutate: func(f ObjectFunctions) error {
				return f.WriteFile(context.Background(), "obj_001", "data.txt", []byte("data"))
			},
			expectedFiles: map[string]int64{"data.txt": 4},
		},
		{
			name: "append",
			mutate: func(f ObjectFunctions) error {
				return f.AppendFile(context.Background(), "obj_001", "data.txt", []byte("more"))
			},
			expectedFiles: map[string]int64{"data.txt": 8},
		},
		{
			name: "delete",
			mutate: func(f ObjectFunctions) error {
				return f.DeleteFile(context.Background(), "obj_001", "data.txt")
			},
			expectedFiles: map[string]int64{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var manifestFile model.ObjectManifest
			var cachedManifest *model.ObjectManifest
			publisher := &capturePublisher{}
			pg := &fakeObjectStore{
				getFn: func(context.Context, string) (*model.Object, error) {
					return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system", Version: 7}, nil
				},
				updateManifestFn: func(_ context.Context, _ string, manifest *model.ObjectManifest, _ ...time.Time) error {
					cachedManifest = model.NormalizeManifest(manifest)
					return nil
				},
			}
			storage := fakeObjectStorage{
				writeFileFn:  func(string, string, []byte) error { return nil },
				appendFileFn: func(string, string, []byte) error { return nil },
				deleteFileFn: func(string, string) error { return nil },
				listFilesFn: func(string) ([]string, error) {
					names := make([]string, 0, len(tc.expectedFiles))
					for name := range tc.expectedFiles {
						names = append(names, name)
					}
					sort.Strings(names)
					return names, nil
				},
				fileInfoFn: func(_ string, filename string) (model.ObjectFileInfo, error) {
					return model.ObjectFileInfo{
						Size:      tc.expectedFiles[filename],
						UpdatedAt: mustParseTime(t, "2026-05-05T00:00:00Z"),
					}, nil
				},
				writeManifestFn: func(_ string, data []byte) error {
					if err := json.Unmarshal(data, &manifestFile); err != nil {
						t.Fatalf("manifest write was not valid JSON: %v", err)
					}
					return nil
				},
			}
			f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator(), publisher)
			if err := tc.mutate(f); err != nil {
				t.Fatalf("%s failed: %v", tc.name, err)
			}
			if cachedManifest == nil {
				t.Fatal("expected database manifest cache update")
			}
			if len(manifestFile.Files) != len(tc.expectedFiles) {
				t.Fatalf("expected filesystem manifest files %+v, got %+v", tc.expectedFiles, manifestFile.Files)
			}
			if len(cachedManifest.Files) != len(tc.expectedFiles) {
				t.Fatalf("expected cached manifest files %+v, got %+v", tc.expectedFiles, cachedManifest.Files)
			}
			for name, size := range tc.expectedFiles {
				if manifestFile.Files[name].Size != size {
					t.Fatalf("expected filesystem manifest size %d for %s, got %+v", size, name, manifestFile.Files[name])
				}
				if cachedManifest.Files[name].Size != size {
					t.Fatalf("expected cached manifest size %d for %s, got %+v", size, name, cachedManifest.Files[name])
				}
			}
			if len(publisher.events) != 1 {
				t.Fatalf("expected one object mutation event, got %d", len(publisher.events))
			}
			event := publisher.events[0]
			if event.GetResource() != "object" || event.GetOperation() != "updated" || event.GetResourceId() != "obj_001" {
				t.Fatalf("unexpected mutation event: %+v", event)
			}
			if event.GetResourceVersion() != 7 {
				t.Fatalf("expected object event version 7, got %d", event.GetResourceVersion())
			}
			if event.GetObject().GetVersion() != 7 {
				t.Fatalf("expected object snapshot version 7, got %d", event.GetObject().GetVersion())
			}
		})
	}
}

func TestObjectFunctions_UpdateObjectManifestPublishesMutation(t *testing.T) {
	publisher := &capturePublisher{}
	pg := &fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system", Version: 7}, nil
		},
	}
	storage := fakeObjectStorage{}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator(), publisher)

	manifest := model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{
		"data.txt": {Size: 4, UpdatedAt: mustParseTime(t, "2026-05-05T00:00:00Z")},
	}})
	if err := f.UpdateObjectManifest(context.Background(), "obj_001", manifest); err != nil {
		t.Fatalf("update manifest failed: %v", err)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one object mutation event, got %d", len(publisher.events))
	}
	event := publisher.events[0]
	if event.GetResource() != "object" || event.GetOperation() != "updated" || event.GetResourceId() != "obj_001" {
		t.Fatalf("unexpected mutation event: %+v", event)
	}
	if event.GetObject().GetVersion() != 7 {
		t.Fatalf("expected object snapshot version 7, got %d", event.GetObject().GetVersion())
	}
}

func TestObjectFunctions_ReconcileRepairsMissingManifest(t *testing.T) {
	var repairedManifest []byte
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
		listFilesFn:   func(string) ([]string, error) { return []string{"data.txt"}, nil },
		fileInfoFn: func(string, string) (model.ObjectFileInfo, error) {
			return model.ObjectFileInfo{Size: 4, UpdatedAt: mustParseTime(t, "2026-05-05T00:00:00Z")}, nil
		},
		readManifestFn: func(string) ([]byte, error) {
			if !repaired {
				return nil, model.ErrNotFound
			}
			return repairedManifest, nil
		},
		writeManifestFn: func(_ string, data []byte) error {
			repaired = true
			repairedManifest = append([]byte(nil), data...)
			return nil
		},
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator())
	if err := f.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !repaired {
		t.Fatal("expected reconcile to repair missing manifest")
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(repairedManifest, &manifest); err != nil {
		t.Fatalf("expected repaired manifest JSON, got %v", err)
	}
	if manifest.Files["data.txt"].Size != 4 {
		t.Fatalf("expected repaired manifest to retain file index, got %+v", manifest.Files)
	}
}

func TestObjectFunctions_ReconcileRepairsMalformedManifestWithoutErasingFiles(t *testing.T) {
	var repairedManifest []byte
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
		listFilesFn:   func(string) ([]string, error) { return []string{"data.txt"}, nil },
		fileInfoFn: func(string, string) (model.ObjectFileInfo, error) {
			return model.ObjectFileInfo{Size: 7, UpdatedAt: mustParseTime(t, "2026-05-06T00:00:00Z")}, nil
		},
		readManifestFn: func(string) ([]byte, error) {
			if repairedManifest != nil {
				return repairedManifest, nil
			}
			return []byte(`{"files":`), nil
		},
		writeManifestFn: func(_ string, data []byte) error {
			repairedManifest = append([]byte(nil), data...)
			return nil
		},
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger(), testProtoValidator())
	if err := f.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(repairedManifest, &manifest); err != nil {
		t.Fatalf("expected repaired manifest JSON, got %v", err)
	}
	if manifest.Files["data.txt"].Size != 7 {
		t.Fatalf("expected repaired manifest to retain malformed-manifest file index, got %+v", manifest.Files)
	}
}

func TestModelErrors_IsCoreError(t *testing.T) {
	wrapped := fmt.Errorf("wrapped: %w", model.ErrNotFound)
	if !errors.Is(wrapped, model.ErrNotFound) {
		t.Fatal("expected wrapped core error to match with errors.Is")
	}
}
