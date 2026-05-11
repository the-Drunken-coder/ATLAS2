package service

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/runtime/config"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

func testLogger() *logging.Logger {
	return logging.New(&config.Config{LogLevel: "debug"}, "test")
}

func validAssetJSON() []byte {
	return []byte(`{"components":{"supported_commands":{"commands":["move_to_location"]}},"extra":{}}`)
}

func validTaskJSON() []byte {
	return []byte(`{"components":{"command":{"type":"move_to_location"},"parameters":{}},"extra":{}}`)
}

func validCommandCatalogJSON() []byte {
	return []byte(`{"commands":{"move_to_location":{"parameters_schema":{"type":"object","properties":{"latitude":{"type":"number"},"mode":{"type":"string","enum":["auto","manual"]}},"required":[],"additionalProperties":false}}}}`)
}

func validCommandCatalogObject() *model.Object {
	return &model.Object{ObjectID: "command_catalog", Type: model.ObjectTypeDocument, JSON: validCommandCatalogJSON()}
}

func validObservationJSON() []byte {
	return []byte(`{"state":"active","extra":{}}`)
}

type fakeEntityStore struct {
	createFn func(context.Context, *model.Entity) error
	getFn    func(context.Context, string) (*model.Entity, error)
	updateFn func(context.Context, *model.Entity) error
	upsertFn func(context.Context, *model.Entity) error
}

func (s fakeEntityStore) CreateEntity(ctx context.Context, entity *model.Entity) error {
	if s.createFn != nil {
		return s.createFn(ctx, entity)
	}
	return nil
}
func (s fakeEntityStore) GetEntity(ctx context.Context, entityID string) (*model.Entity, error) {
	if s.getFn != nil {
		return s.getFn(ctx, entityID)
	}
	return nil, model.ErrNotFound
}
func (fakeEntityStore) ListEntities(context.Context, ...ports.EntityFilter) ([]model.Entity, error) {
	return nil, nil
}
func (s fakeEntityStore) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, entity)
	}
	return nil
}
func (fakeEntityStore) DeleteEntity(context.Context, string) error { return nil }
func (s fakeEntityStore) UpsertEntity(ctx context.Context, entity *model.Entity) error {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, entity)
	}
	return nil
}

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
func (s fakeTaskStore) ListTasks(context.Context, ...ports.TaskFilter) ([]model.Task, error) {
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

type fakeObservationStore struct {
	createFn func(context.Context, *model.Observation) error
	getFn    func(context.Context, string) (*model.Observation, error)
	updateFn func(context.Context, *model.Observation) error
	upsertFn func(context.Context, *model.Observation) error
}

func (s fakeObservationStore) CreateObservation(ctx context.Context, obs *model.Observation) error {
	if s.createFn != nil {
		return s.createFn(ctx, obs)
	}
	return nil
}
func (s fakeObservationStore) GetObservation(ctx context.Context, observationID string) (*model.Observation, error) {
	if s.getFn != nil {
		return s.getFn(ctx, observationID)
	}
	return nil, model.ErrNotFound
}
func (fakeObservationStore) ListObservations(context.Context, ...ports.ObservationFilter) ([]model.Observation, error) {
	return nil, nil
}
func (s fakeObservationStore) UpdateObservation(ctx context.Context, obs *model.Observation) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, obs)
	}
	return nil
}
func (fakeObservationStore) DeleteObservation(context.Context, string) error { return nil }
func (s fakeObservationStore) UpsertObservation(ctx context.Context, obs *model.Observation) error {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, obs)
	}
	return nil
}

type fakeObjectStore struct {
	createFn             func(context.Context, *model.Object) error
	getFn                func(context.Context, string) (*model.Object, error)
	listFn               func(context.Context, ...ports.ObjectFilter) ([]model.Object, error)
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
func (s *fakeObjectStore) ListObjects(ctx context.Context, filters ...ports.ObjectFilter) ([]model.Object, error) {
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
	tryBeginFn      func(context.Context, string, string, string) (ports.IdempotencyRecord, bool, error)
	markCompletedFn func(context.Context, string, string) error
	markFailedFn    func(context.Context, string, string) error
}

func (s fakeIdempotencyStore) TryBegin(ctx context.Context, scope, key, resourceID string) (ports.IdempotencyRecord, bool, error) {
	if s.tryBeginFn != nil {
		return s.tryBeginFn(ctx, scope, key, resourceID)
	}
	return ports.IdempotencyRecord{ResourceID: resourceID, Status: ports.IdempotencyStatusPending}, true, nil
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
	ctx := context.Background()
	entity := &model.Entity{Type: model.EntityTypeAsset, JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateEntity(ctx, entity); err == nil {
		t.Fatal("expected error for empty entity_id")
	}
	entity.EntityID = "this-entity-id-is-way-too-long-for-the-50-character-limit"
	if err := f.CreateEntity(ctx, entity); err == nil {
		t.Fatal("expected error for long entity_id")
	}
}

func TestEntityFunctions_ValidateType(t *testing.T) {
	f := EntityFunctions{}
	ctx := context.Background()
	entity := &model.Entity{EntityID: "test_001", Type: model.EntityType("invalid_type"), JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateEntity(ctx, entity); err == nil {
		t.Fatal("expected error for invalid type")
	}
}
