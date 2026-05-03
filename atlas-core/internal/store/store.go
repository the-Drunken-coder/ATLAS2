package store

import (
	"context"
	"io"

	"github.com/anomalyco/atlas-core/internal/model"
)

type EntityStore interface {
	CreateEntity(ctx context.Context, entity *model.Entity) error
	GetEntity(ctx context.Context, entityID string) (*model.Entity, error)
	ListEntities(ctx context.Context, filters ...EntityFilter) ([]model.Entity, error)
	UpdateEntity(ctx context.Context, entity *model.Entity) error
	DeleteEntity(ctx context.Context, entityID string) error
	UpsertEntity(ctx context.Context, entity *model.Entity) error
}

type EntityFilter func(*EntityFilterState)

type EntityFilterState struct {
	EntityType   *model.EntityType
	UpdatedAfter *string
}

func WithEntityType(t model.EntityType) EntityFilter {
	return func(f *EntityFilterState) {
		f.EntityType = &t
	}
}

func WithEntityUpdatedAfter(ts string) EntityFilter {
	return func(f *EntityFilterState) {
		f.UpdatedAfter = &ts
	}
}

type ObjectStore interface {
	CreateObject(ctx context.Context, obj *model.Object) error
	GetObject(ctx context.Context, objectID string) (*model.Object, error)
	ListObjects(ctx context.Context, filters ...ObjectFilter) ([]model.Object, error)
	UpdateObject(ctx context.Context, obj *model.Object) error
	DeleteObject(ctx context.Context, objectID string) error
	UpsertObject(ctx context.Context, obj *model.Object) error
	UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest) error
	GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error)
}

type ObjectFilter func(*ObjectFilterState)

type ObjectFilterState struct {
	OwnerType    *model.OwnerType
	OwnerID      *string
	ObjectType   *string
	UpdatedAfter *string
}

func WithObjectOwnerType(t model.OwnerType) ObjectFilter {
	return func(f *ObjectFilterState) {
		f.OwnerType = &t
	}
}

func WithObjectOwnerID(id string) ObjectFilter {
	return func(f *ObjectFilterState) {
		f.OwnerID = &id
	}
}

func WithObjectType(t string) ObjectFilter {
	return func(f *ObjectFilterState) {
		f.ObjectType = &t
	}
}

func WithObjectUpdatedAfter(ts string) ObjectFilter {
	return func(f *ObjectFilterState) {
		f.UpdatedAfter = &ts
	}
}

type TaskStore interface {
	CreateTask(ctx context.Context, task *model.Task) error
	GetTask(ctx context.Context, taskID string) (*model.Task, error)
	ListTasks(ctx context.Context, filters ...TaskFilter) ([]model.Task, error)
	UpdateTask(ctx context.Context, task *model.Task) error
	DeleteTask(ctx context.Context, taskID string) error
	UpsertTask(ctx context.Context, task *model.Task) error
}

type TaskFilter func(*TaskFilterState)

type TaskFilterState struct {
	AssetID      *string
	Status       *model.TaskStatus
	UpdatedAfter *string
}

func WithTaskAssetID(id string) TaskFilter {
	return func(f *TaskFilterState) {
		f.AssetID = &id
	}
}

func WithTaskStatus(s model.TaskStatus) TaskFilter {
	return func(f *TaskFilterState) {
		f.Status = &s
	}
}

func WithTaskUpdatedAfter(ts string) TaskFilter {
	return func(f *TaskFilterState) {
		f.UpdatedAfter = &ts
	}
}

type ObservationStore interface {
	CreateObservation(ctx context.Context, obs *model.Observation) error
	GetObservation(ctx context.Context, observationID string) (*model.Observation, error)
	ListObservations(ctx context.Context, filters ...ObservationFilter) ([]model.Observation, error)
	UpdateObservation(ctx context.Context, obs *model.Observation) error
	DeleteObservation(ctx context.Context, observationID string) error
	UpsertObservation(ctx context.Context, obs *model.Observation) error
}

type ObservationFilter func(*ObservationFilterState)

type ObservationFilterState struct {
	SourceAssetID *string
	UpdatedAfter  *string
}

func WithObservationSourceAssetID(id string) ObservationFilter {
	return func(f *ObservationFilterState) {
		f.SourceAssetID = &id
	}
}

func WithObservationUpdatedAfter(ts string) ObservationFilter {
	return func(f *ObservationFilterState) {
		f.UpdatedAfter = &ts
	}
}

type ObjectStorageStore interface {
	CreateObjectFolder(objectID string) error
	ObjectFolderExists(objectID string) (bool, error)
	DeleteObjectFolder(objectID string) error
	WriteObjectFile(objectID, filename string, data []byte) error
	AppendObjectFile(objectID, filename string, data []byte) error
	ReadObjectFile(objectID, filename string) ([]byte, error)
	DeleteObjectFile(objectID, filename string) error
	ListObjectFolderFiles(objectID string) ([]string, error)
	ReadManifestFile(objectID string) ([]byte, error)
	WriteManifestFile(objectID string, data []byte) error
	ValidateSafeObjectPath(objectID, filename string) error
	ReaderForObjectFile(objectID, filename string) (io.ReadCloser, error)
}
