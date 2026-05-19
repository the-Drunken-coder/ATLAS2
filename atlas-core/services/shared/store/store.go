package store

import (
	"context"
	"io"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

type EntityListParams struct {
	Filters   []EntityFilter
	PageSize  int32
	PageToken string
}

type EntityListResult struct {
	Entities      []model.Entity
	NextPageToken string
}

type EntityStore interface {
	CreateEntity(ctx context.Context, entity *model.Entity) error
	GetEntity(ctx context.Context, entityID string) (*model.Entity, error)
	ListEntities(ctx context.Context, params EntityListParams) (EntityListResult, error)
	UpdateEntity(ctx context.Context, entity *model.Entity) error
	DeleteEntity(ctx context.Context, entityID string) error
	UpsertEntity(ctx context.Context, entity *model.Entity) error
}

type EntityFilter func(*EntityFilterState)

type EntityFilterState struct {
	EntityType   *model.EntityType
	UpdatedAfter *time.Time
}

func WithEntityType(t model.EntityType) EntityFilter {
	return func(f *EntityFilterState) { f.EntityType = &t }
}

func WithEntityUpdatedAfter(ts time.Time) EntityFilter {
	return func(f *EntityFilterState) {
		utc := ts.UTC()
		f.UpdatedAfter = &utc
	}
}

type ObjectListParams struct {
	Filters   []ObjectFilter
	PageSize  int32
	PageToken string
}

type ObjectListResult struct {
	Objects       []model.Object
	NextPageToken string
}

type ObjectStore interface {
	CreateObject(ctx context.Context, obj *model.Object) error
	GetObject(ctx context.Context, objectID string) (*model.Object, error)
	ListObjects(ctx context.Context, params ObjectListParams) (ObjectListResult, error)
	UpdateObject(ctx context.Context, obj *model.Object) error
	DeleteObject(ctx context.Context, objectID string) error
	UpsertObject(ctx context.Context, obj *model.Object) error
	UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest, updatedAt ...time.Time) error
	GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error)
}

type ObjectFilter func(*ObjectFilterState)

type ObjectFilterState struct {
	OwnerType    *model.OwnerType
	OwnerID      *string
	ObjectType   *model.ObjectType
	UpdatedAfter *time.Time
}

func WithObjectOwnerType(t model.OwnerType) ObjectFilter {
	return func(f *ObjectFilterState) { f.OwnerType = &t }
}

func WithObjectOwnerID(id string) ObjectFilter {
	return func(f *ObjectFilterState) { f.OwnerID = &id }
}

func WithObjectOwner(ownerType model.OwnerType, ownerID string) ObjectFilter {
	return func(f *ObjectFilterState) {
		f.OwnerType = &ownerType
		f.OwnerID = &ownerID
	}
}

func WithObjectType(t model.ObjectType) ObjectFilter {
	return func(f *ObjectFilterState) { f.ObjectType = &t }
}

func WithObjectUpdatedAfter(ts time.Time) ObjectFilter {
	return func(f *ObjectFilterState) {
		utc := ts.UTC()
		f.UpdatedAfter = &utc
	}
}

type TaskListParams struct {
	Filters   []TaskFilter
	PageSize  int32
	PageToken string
}

type TaskListResult struct {
	Tasks         []model.Task
	NextPageToken string
}

type TaskStore interface {
	CreateTask(ctx context.Context, task *model.Task) error
	GetTask(ctx context.Context, taskID string) (*model.Task, error)
	ListTasks(ctx context.Context, params TaskListParams) (TaskListResult, error)
	UpdateTask(ctx context.Context, task *model.Task) error
	DeleteTask(ctx context.Context, taskID string) error
	UpsertTask(ctx context.Context, task *model.Task) error
}

type TaskFilter func(*TaskFilterState)

type TaskFilterState struct {
	AssetID      *string
	Status       *model.TaskStatus
	UpdatedAfter *time.Time
}

func WithTaskAssetID(id string) TaskFilter {
	return func(f *TaskFilterState) { f.AssetID = &id }
}

func WithTaskStatus(s model.TaskStatus) TaskFilter {
	return func(f *TaskFilterState) { f.Status = &s }
}

func WithTaskUpdatedAfter(ts time.Time) TaskFilter {
	return func(f *TaskFilterState) {
		utc := ts.UTC()
		f.UpdatedAfter = &utc
	}
}

type ObservationListParams struct {
	Filters   []ObservationFilter
	PageSize  int32
	PageToken string
}

type ObservationListResult struct {
	Observations  []model.Observation
	NextPageToken string
}

type ObservationStore interface {
	CreateObservation(ctx context.Context, obs *model.Observation) error
	GetObservation(ctx context.Context, observationID string) (*model.Observation, error)
	ListObservations(ctx context.Context, params ObservationListParams) (ObservationListResult, error)
	UpdateObservation(ctx context.Context, obs *model.Observation) error
	DeleteObservation(ctx context.Context, observationID string) error
	UpsertObservation(ctx context.Context, obs *model.Observation) error
}

type ObservationFilter func(*ObservationFilterState)

type ObservationFilterState struct {
	SourceAssetID  *string
	TargetEntityID *string
	ObservedAtFrom *time.Time
	ObservedAtTo   *time.Time
	UpdatedAfter   *time.Time
}

func WithObservationSourceAssetID(id string) ObservationFilter {
	return func(f *ObservationFilterState) { f.SourceAssetID = &id }
}

func WithObservationTargetEntityID(id string) ObservationFilter {
	return func(f *ObservationFilterState) { f.TargetEntityID = &id }
}

func WithObservationObservedAtFrom(ts time.Time) ObservationFilter {
	return func(f *ObservationFilterState) {
		utc := ts.UTC()
		f.ObservedAtFrom = &utc
	}
}

func WithObservationObservedAtTo(ts time.Time) ObservationFilter {
	return func(f *ObservationFilterState) {
		utc := ts.UTC()
		f.ObservedAtTo = &utc
	}
}

func WithObservationUpdatedAfter(ts time.Time) ObservationFilter {
	return func(f *ObservationFilterState) {
		utc := ts.UTC()
		f.UpdatedAfter = &utc
	}
}

type IdempotencyStatus string

const (
	IdempotencyStatusPending   IdempotencyStatus = "pending"
	IdempotencyStatusCompleted IdempotencyStatus = "completed"
	IdempotencyStatusFailed    IdempotencyStatus = "failed"
)

type IdempotencyRecord struct {
	ResourceID string
	Status     IdempotencyStatus
}

// IdempotencyStore records dedup state for operations that the caller wants
// to make safely retryable. Scope partitions keys by operation class (e.g.
// "object_create", "task_create"). TryBegin returns claimed=true when the
// caller now owns a pending claim for the operation, and claimed=false with the
// currently stored record otherwise.
type IdempotencyStore interface {
	TryBegin(ctx context.Context, scope, key, resourceID string) (record IdempotencyRecord, claimed bool, err error)
	MarkCompleted(ctx context.Context, scope, key string) error
	MarkFailed(ctx context.Context, scope, key string) error
}

type ObjectStorageStore interface {
	CreateObjectFolder(objectID string) error
	ObjectFolderExists(objectID string) (bool, error)
	ListObjectFolders() ([]string, error)
	DeleteObjectFolder(objectID string) error
	RenameObjectFolder(objectID, newName string) error
	WriteObjectFile(objectID, filename string, data []byte) error
	AppendObjectFile(objectID, filename string, data []byte) error
	ReadObjectFile(objectID, filename string) ([]byte, error)
	DeleteObjectFile(objectID, filename string) error
	ListObjectFolderFiles(objectID string) ([]string, error)
	GetObjectFileInfo(objectID, filename string) (model.ObjectFileInfo, error)
	ReadManifestFile(objectID string) ([]byte, error)
	WriteManifestFile(objectID string, data []byte) error
	ValidateSafeObjectPath(objectID, filename string) error
	ReaderForObjectFile(objectID, filename string) (io.ReadCloser, error)
}
