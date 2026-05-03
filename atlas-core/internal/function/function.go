package function

import (
	"context"
	"time"

	"encoding/json"

	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/objectstorage"
	"github.com/anomalyco/atlas-core/internal/postgres"
	"github.com/anomalyco/atlas-core/internal/store"
)

type Functions struct {
	Entity      EntityFunctions
	Object      ObjectFunctions
	Task        TaskFunctions
	Observation ObservationFunctions
}

type EntityFunctions struct {
	pgStore *postgres.EntityStore
	log     *logging.Logger
}

func NewEntityFunctions(pgStore *postgres.EntityStore, log *logging.Logger) EntityFunctions {
	return EntityFunctions{pgStore: pgStore, log: log}
}

func (f EntityFunctions) CreateEntity(ctx context.Context, entity *model.Entity) error {
	if err := requireModel(entity, "entity"); err != nil {
		return err
	}
	if entity.EntityID == "" {
		return model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	if len(entity.EntityID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "entity_id must be 1-50 characters", "entity_id")
	}
	if entity.Type != model.EntityTypeAsset && entity.Type != model.EntityTypeTrack && entity.Type != model.EntityTypeGeofeature {
		return model.NewFieldError("INVALID_INPUT", "type must be asset, track, or geofeature", "type")
	}
	now := time.Now().UTC()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	if entity.UpdatedAt.IsZero() {
		entity.UpdatedAt = now
	}
	if entity.JSON == nil {
		entity.JSON = []byte("{}")
	}
	f.log.Info("entity", "creating entity "+entity.EntityID)
	return f.pgStore.CreateEntity(ctx, entity)
}

func (f EntityFunctions) GetEntity(ctx context.Context, entityID string) (*model.Entity, error) {
	if entityID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	return f.pgStore.GetEntity(ctx, entityID)
}

func (f EntityFunctions) ListEntities(ctx context.Context, filters ...store.EntityFilter) ([]model.Entity, error) {
	return f.pgStore.ListEntities(ctx, filters...)
}

func (f EntityFunctions) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	if err := requireModel(entity, "entity"); err != nil {
		return err
	}
	if entity.EntityID == "" {
		return model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	entity.UpdatedAt = time.Now().UTC()
	f.log.Info("entity", "updating entity "+entity.EntityID)
	return f.pgStore.UpdateEntity(ctx, entity)
}

func (f EntityFunctions) DeleteEntity(ctx context.Context, entityID string) error {
	if entityID == "" {
		return model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	f.log.Info("entity", "deleting entity "+entityID)
	return f.pgStore.DeleteEntity(ctx, entityID)
}

func (f EntityFunctions) UpsertEntity(ctx context.Context, entity *model.Entity) error {
	if err := requireModel(entity, "entity"); err != nil {
		return err
	}
	if entity.EntityID == "" {
		return model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	now := time.Now().UTC()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	entity.UpdatedAt = now
	if entity.JSON == nil {
		entity.JSON = []byte("{}")
	}
	f.log.Info("entity", "upserting entity "+entity.EntityID)
	return f.pgStore.UpsertEntity(ctx, entity)
}

type ObjectFunctions struct {
	pgStore  *postgres.ObjectStore
	objStore *objectstorage.Store
	log      *logging.Logger
}

func NewObjectFunctions(pgStore *postgres.ObjectStore, objStore *objectstorage.Store, log *logging.Logger) ObjectFunctions {
	return ObjectFunctions{pgStore: pgStore, objStore: objStore, log: log}
}

func (f ObjectFunctions) CreateObject(ctx context.Context, obj *model.Object) error {
	if err := requireModel(obj, "object"); err != nil {
		return err
	}
	if obj.ObjectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	if len(obj.ObjectID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "object_id must be 1-50 characters", "object_id")
	}
	if obj.Type == "" {
		return model.NewFieldError("INVALID_INPUT", "type is required", "type")
	}
	if obj.OwnerType == "" {
		return model.NewFieldError("INVALID_INPUT", "owner_type is required", "owner_type")
	}
	if obj.OwnerID == "" {
		return model.NewFieldError("INVALID_INPUT", "owner_id is required", "owner_id")
	}
	if err := objectstorage.ValidateObjectID(obj.ObjectID); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	now := time.Now().UTC()
	if obj.CreatedAt.IsZero() {
		obj.CreatedAt = now
	}
	if obj.UpdatedAt.IsZero() {
		obj.UpdatedAt = now
	}
	if obj.JSON == nil {
		obj.JSON = []byte("{}")
	}
	f.log.Info("object", "creating object "+obj.ObjectID)
	if err := f.pgStore.CreateObject(ctx, obj); err != nil {
		return err
	}
	if err := f.objStore.CreateObjectFolder(obj.ObjectID); err != nil {
		if cleanupErr := f.pgStore.DeleteObject(ctx, obj.ObjectID); cleanupErr != nil {
			return model.NewCoreError("OBJECT_CREATE_ERROR", "failed to initialize object storage and rollback metadata: "+cleanupErr.Error())
		}
		return err
	}
	return nil
}

func (f ObjectFunctions) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	if objectID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	return f.pgStore.GetObject(ctx, objectID)
}

func (f ObjectFunctions) ListObjects(ctx context.Context, filters ...store.ObjectFilter) ([]model.Object, error) {
	return f.pgStore.ListObjects(ctx, filters...)
}

func (f ObjectFunctions) UpdateObject(ctx context.Context, obj *model.Object) error {
	if err := requireModel(obj, "object"); err != nil {
		return err
	}
	if obj.ObjectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	obj.UpdatedAt = time.Now().UTC()
	f.log.Info("object", "updating object "+obj.ObjectID)
	return f.pgStore.UpdateObject(ctx, obj)
}

func (f ObjectFunctions) DeleteObject(ctx context.Context, objectID string) error {
	if objectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	obj, err := f.pgStore.GetObject(ctx, objectID)
	if err != nil {
		return err
	}
	f.log.Info("object", "deleting object "+objectID)
	if err := f.pgStore.DeleteObject(ctx, objectID); err != nil {
		return err
	}
	if err := f.objStore.DeleteObjectFolder(objectID); err != nil {
		if restoreErr := f.pgStore.UpsertObject(ctx, obj); restoreErr != nil {
			return model.NewCoreError("OBJECT_DELETE_ERROR", "failed to delete object storage and restore metadata: "+restoreErr.Error())
		}
		return err
	}
	return nil
}

func (f ObjectFunctions) UpsertObject(ctx context.Context, obj *model.Object) error {
	if err := requireModel(obj, "object"); err != nil {
		return err
	}
	if obj.ObjectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	if err := objectstorage.ValidateObjectID(obj.ObjectID); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	now := time.Now().UTC()
	if obj.CreatedAt.IsZero() {
		obj.CreatedAt = now
	}
	obj.UpdatedAt = now
	if obj.JSON == nil {
		obj.JSON = []byte("{}")
	}
	_, existingErr := f.pgStore.GetObject(ctx, obj.ObjectID)
	objectExists := existingErr == nil
	if existingErr != nil && existingErr != model.ErrNotFound {
		return existingErr
	}
	f.log.Info("object", "upserting object "+obj.ObjectID)
	if err := f.pgStore.UpsertObject(ctx, obj); err != nil {
		return err
	}
	folderExists, err := f.objStore.ObjectFolderExists(obj.ObjectID)
	if err != nil {
		if !objectExists {
			_ = f.pgStore.DeleteObject(ctx, obj.ObjectID)
		}
		return err
	}
	if !folderExists {
		if err := f.objStore.CreateObjectFolder(obj.ObjectID); err != nil {
			if !objectExists {
				_ = f.pgStore.DeleteObject(ctx, obj.ObjectID)
			}
			return err
		}
	}
	return nil
}

func (f ObjectFunctions) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	if objectID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	return f.pgStore.GetObjectManifest(ctx, objectID)
}

func (f ObjectFunctions) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest) error {
	if objectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	if manifest == nil {
		return model.NewFieldError("INVALID_INPUT", "manifest is required", "manifest")
	}
	if manifest.Files == nil {
		manifest.Files = map[string]model.ObjectFileInfo{}
	}
	previousManifest, err := f.pgStore.GetObjectManifest(ctx, objectID)
	if err != nil {
		return err
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return model.NewCoreError("MANIFEST_ERROR", "failed to marshal manifest: "+err.Error())
	}
	if err := f.pgStore.UpdateObjectManifest(ctx, objectID, manifest); err != nil {
		return err
	}
	if err := f.objStore.WriteManifestFile(objectID, manifestBytes); err != nil {
		if rollbackErr := f.pgStore.UpdateObjectManifest(ctx, objectID, previousManifest); rollbackErr != nil {
			return model.NewCoreError("MANIFEST_ERROR", "failed to sync manifest to object storage and rollback metadata: "+rollbackErr.Error())
		}
		return err
	}
	return nil
}

func (f ObjectFunctions) WriteFile(objectID, filename string, data []byte) error {
	if err := f.objStore.ValidateSafeObjectPath(objectID, filename); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	f.log.Info("object", "writing file "+filename+" in object "+objectID)
	return f.objStore.WriteObjectFile(objectID, filename, data)
}

func (f ObjectFunctions) AppendFile(objectID, filename string, data []byte) error {
	if err := f.objStore.ValidateSafeObjectPath(objectID, filename); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	f.log.Info("object", "appending to file "+filename+" in object "+objectID)
	return f.objStore.AppendObjectFile(objectID, filename, data)
}

func (f ObjectFunctions) ReadFile(objectID, filename string) ([]byte, error) {
	if err := f.objStore.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	return f.objStore.ReadObjectFile(objectID, filename)
}

func (f ObjectFunctions) DeleteFile(objectID, filename string) error {
	if err := f.objStore.ValidateSafeObjectPath(objectID, filename); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	f.log.Info("object", "deleting file "+filename+" from object "+objectID)
	return f.objStore.DeleteObjectFile(objectID, filename)
}

func (f ObjectFunctions) ListFiles(objectID string) ([]string, error) {
	return f.objStore.ListObjectFolderFiles(objectID)
}

type TaskFunctions struct {
	pgStore *postgres.TaskStore
	log     *logging.Logger
}

func NewTaskFunctions(pgStore *postgres.TaskStore, log *logging.Logger) TaskFunctions {
	return TaskFunctions{pgStore: pgStore, log: log}
}

func (f TaskFunctions) CreateTask(ctx context.Context, task *model.Task) error {
	if err := requireModel(task, "task"); err != nil {
		return err
	}
	if task.TaskID == "" {
		return model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	if len(task.TaskID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "task_id must be 1-50 characters", "task_id")
	}
	if task.Status == "" {
		return model.NewFieldError("INVALID_INPUT", "status is required", "status")
	}
	if task.AssetID == "" {
		return model.NewFieldError("INVALID_INPUT", "asset_id is required", "asset_id")
	}
	if task.CommandCatalogObjectID == "" {
		return model.NewFieldError("INVALID_INPUT", "command_catalog_object_id is required", "command_catalog_object_id")
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}
	if task.JSON == nil {
		task.JSON = []byte("{}")
	}
	f.log.Info("task", "creating task "+task.TaskID)
	return f.pgStore.CreateTask(ctx, task)
}

func (f TaskFunctions) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	if taskID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	return f.pgStore.GetTask(ctx, taskID)
}

func (f TaskFunctions) ListTasks(ctx context.Context, filters ...store.TaskFilter) ([]model.Task, error) {
	return f.pgStore.ListTasks(ctx, filters...)
}

func (f TaskFunctions) UpdateTask(ctx context.Context, task *model.Task) error {
	if err := requireModel(task, "task"); err != nil {
		return err
	}
	if task.TaskID == "" {
		return model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	task.UpdatedAt = time.Now().UTC()
	f.log.Info("task", "updating task "+task.TaskID)
	return f.pgStore.UpdateTask(ctx, task)
}

func (f TaskFunctions) DeleteTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	f.log.Info("task", "deleting task "+taskID)
	return f.pgStore.DeleteTask(ctx, taskID)
}

func (f TaskFunctions) UpsertTask(ctx context.Context, task *model.Task) error {
	if err := requireModel(task, "task"); err != nil {
		return err
	}
	if task.TaskID == "" {
		return model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	if task.JSON == nil {
		task.JSON = []byte("{}")
	}
	f.log.Info("task", "upserting task "+task.TaskID)
	return f.pgStore.UpsertTask(ctx, task)
}

type ObservationFunctions struct {
	pgStore *postgres.ObservationStore
	log     *logging.Logger
}

func NewObservationFunctions(pgStore *postgres.ObservationStore, log *logging.Logger) ObservationFunctions {
	return ObservationFunctions{pgStore: pgStore, log: log}
}

func (f ObservationFunctions) CreateObservation(ctx context.Context, obs *model.Observation) error {
	if err := requireModel(obs, "observation"); err != nil {
		return err
	}
	if obs.ObservationID == "" {
		return model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	if len(obs.ObservationID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "observation_id must be 1-50 characters", "observation_id")
	}
	if obs.SourceAssetID == "" {
		return model.NewFieldError("INVALID_INPUT", "source_asset_id is required", "source_asset_id")
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	if obs.UpdatedAt.IsZero() {
		obs.UpdatedAt = now
	}
	if obs.JSON == nil {
		obs.JSON = []byte("{}")
	}
	f.log.Info("observation", "creating observation "+obs.ObservationID)
	return f.pgStore.CreateObservation(ctx, obs)
}

func (f ObservationFunctions) GetObservation(ctx context.Context, observationID string) (*model.Observation, error) {
	if observationID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	return f.pgStore.GetObservation(ctx, observationID)
}

func (f ObservationFunctions) ListObservations(ctx context.Context, filters ...store.ObservationFilter) ([]model.Observation, error) {
	return f.pgStore.ListObservations(ctx, filters...)
}

func (f ObservationFunctions) UpdateObservation(ctx context.Context, obs *model.Observation) error {
	if err := requireModel(obs, "observation"); err != nil {
		return err
	}
	if obs.ObservationID == "" {
		return model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	obs.UpdatedAt = time.Now().UTC()
	f.log.Info("observation", "updating observation "+obs.ObservationID)
	return f.pgStore.UpdateObservation(ctx, obs)
}

func (f ObservationFunctions) DeleteObservation(ctx context.Context, observationID string) error {
	if observationID == "" {
		return model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	f.log.Info("observation", "deleting observation "+observationID)
	return f.pgStore.DeleteObservation(ctx, observationID)
}

func (f ObservationFunctions) UpsertObservation(ctx context.Context, obs *model.Observation) error {
	if err := requireModel(obs, "observation"); err != nil {
		return err
	}
	if obs.ObservationID == "" {
		return model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	obs.UpdatedAt = now
	if obs.JSON == nil {
		obs.JSON = []byte("{}")
	}
	f.log.Info("observation", "upserting observation "+obs.ObservationID)
	return f.pgStore.UpsertObservation(ctx, obs)
}

func requireModel[T any](value *T, field string) error {
	if value == nil {
		return model.NewFieldError("INVALID_INPUT", field+" is required", field)
	}
	return nil
}
