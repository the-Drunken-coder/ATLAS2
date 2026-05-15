package datastorageclient

import (
	"context"
	"fmt"
	"io"
	"time"

	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type Bundle struct {
	Entity      *EntityStoreClient
	Object      *ObjectGatewayClient
	Task        *TaskStoreClient
	Observation *ObservationStoreClient
	Idempotency *IdempotencyStoreClient
}

func New(client datastoragev1.DataStorageServiceClient) Bundle {
	return Bundle{
		Entity:      &EntityStoreClient{client: client},
		Object:      &ObjectGatewayClient{client: client},
		Task:        &TaskStoreClient{client: client},
		Observation: &ObservationStoreClient{client: client},
		Idempotency: &IdempotencyStoreClient{client: client},
	}
}

type EntityStoreClient struct {
	client datastoragev1.DataStorageServiceClient
}

type ObjectGatewayClient struct {
	client datastoragev1.DataStorageServiceClient
}

type TaskStoreClient struct {
	client datastoragev1.DataStorageServiceClient
}

type ObservationStoreClient struct {
	client datastoragev1.DataStorageServiceClient
}

type IdempotencyStoreClient struct {
	client datastoragev1.DataStorageServiceClient
}

type NopObjectStorageStore struct{}

var _ functionpkg.ObjectGateway = (*ObjectGatewayClient)(nil)
var _ store.EntityStore = (*EntityStoreClient)(nil)
var _ store.TaskStore = (*TaskStoreClient)(nil)
var _ store.ObservationStore = (*ObservationStoreClient)(nil)
var _ store.IdempotencyStore = (*IdempotencyStoreClient)(nil)

func (c *EntityStoreClient) CreateEntity(ctx context.Context, entity *model.Entity) error {
	resp, err := c.client.CreateEntity(ctx, &sharedv1.EntityRequest{Entity: pbconv.EntityToProto(entity)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.EntityFromProto(resp.GetEntity())
	if convErr == nil {
		*entity = *converted
	}
	return convErr
}
func (c *EntityStoreClient) GetEntity(ctx context.Context, entityID string) (*model.Entity, error) {
	resp, err := c.client.GetEntity(ctx, &sharedv1.GetEntityRequest{EntityId: entityID})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return pbconv.EntityFromProto(resp.GetEntity())
}
func (c *EntityStoreClient) ListEntities(ctx context.Context, filters ...store.EntityFilter) ([]model.Entity, error) {
	resp, err := c.client.ListEntities(ctx, &sharedv1.ListEntitiesRequest{Filter: pbconv.EntityFilterToProto(filters)})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	out := make([]model.Entity, 0, len(resp.GetEntities()))
	for _, entity := range resp.GetEntities() {
		converted, convErr := pbconv.EntityFromProto(entity)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, *converted)
	}
	return out, nil
}
func (c *EntityStoreClient) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	resp, err := c.client.UpdateEntity(ctx, &sharedv1.EntityRequest{Entity: pbconv.EntityToProto(entity)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.EntityFromProto(resp.GetEntity())
	if convErr == nil {
		*entity = *converted
	}
	return convErr
}
func (c *EntityStoreClient) DeleteEntity(ctx context.Context, entityID string) error {
	_, err := c.client.DeleteEntity(ctx, &sharedv1.DeleteEntityRequest{EntityId: entityID})
	return rpcerrors.FromStatus(err)
}
func (c *EntityStoreClient) UpsertEntity(ctx context.Context, entity *model.Entity) error {
	resp, err := c.client.UpsertEntity(ctx, &sharedv1.EntityRequest{Entity: pbconv.EntityToProto(entity)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.EntityFromProto(resp.GetEntity())
	if convErr == nil {
		*entity = *converted
	}
	return convErr
}

func (c *ObjectGatewayClient) CreateObject(ctx context.Context, object *model.Object) error {
	resp, err := c.client.CreateObject(ctx, &sharedv1.ObjectRequest{Object: pbconv.ObjectToProto(object)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.ObjectFromProto(resp.GetObject())
	if convErr == nil {
		*object = *converted
	}
	return convErr
}
func (c *ObjectGatewayClient) EnsureObjectCreated(ctx context.Context, object *model.Object) error {
	resp, err := c.client.EnsureObjectCreated(ctx, &sharedv1.ObjectRequest{Object: pbconv.ObjectToProto(object)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.ObjectFromProto(resp.GetObject())
	if convErr == nil {
		*object = *converted
	}
	return convErr
}
func (c *ObjectGatewayClient) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	resp, err := c.client.GetObject(ctx, &sharedv1.GetObjectRequest{ObjectId: objectID})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return pbconv.ObjectFromProto(resp.GetObject())
}
func (c *ObjectGatewayClient) ListObjects(ctx context.Context, filters ...store.ObjectFilter) ([]model.Object, error) {
	resp, err := c.client.ListObjects(ctx, &sharedv1.ListObjectsRequest{Filter: pbconv.ObjectFilterToProto(filters)})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	out := make([]model.Object, 0, len(resp.GetObjects()))
	for _, object := range resp.GetObjects() {
		converted, convErr := pbconv.ObjectFromProto(object)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, *converted)
	}
	return out, nil
}
func (c *ObjectGatewayClient) UpdateObject(ctx context.Context, object *model.Object) error {
	resp, err := c.client.UpdateObject(ctx, &sharedv1.ObjectRequest{Object: pbconv.ObjectToProto(object)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.ObjectFromProto(resp.GetObject())
	if convErr == nil {
		*object = *converted
	}
	return convErr
}
func (c *ObjectGatewayClient) DeleteObject(ctx context.Context, objectID string) error {
	_, err := c.client.DeleteObject(ctx, &sharedv1.DeleteObjectRequest{ObjectId: objectID})
	return rpcerrors.FromStatus(err)
}
func (c *ObjectGatewayClient) UpsertObject(ctx context.Context, object *model.Object) error {
	resp, err := c.client.UpsertObject(ctx, &sharedv1.ObjectRequest{Object: pbconv.ObjectToProto(object)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.ObjectFromProto(resp.GetObject())
	if convErr == nil {
		*object = *converted
	}
	return convErr
}
func (c *ObjectGatewayClient) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest, _ ...time.Time) error {
	_, err := c.client.UpdateObjectManifest(ctx, &sharedv1.UpdateObjectManifestRequest{ObjectId: objectID, Manifest: pbconv.ManifestToProto(manifest)})
	return rpcerrors.FromStatus(err)
}
func (c *ObjectGatewayClient) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	resp, err := c.client.GetObjectManifest(ctx, &sharedv1.GetObjectManifestRequest{ObjectId: objectID})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return pbconv.ManifestFromProto(resp.GetManifest())
}
func (c *ObjectGatewayClient) WriteFile(ctx context.Context, objectID, filename string, data []byte) error {
	_, err := c.client.WriteObjectFile(ctx, &sharedv1.WriteObjectFileRequest{ObjectId: objectID, Filename: filename, Data: data})
	return rpcerrors.FromStatus(err)
}
func (c *ObjectGatewayClient) AppendFile(ctx context.Context, objectID, filename string, data []byte) error {
	_, err := c.client.AppendObjectFile(ctx, &sharedv1.WriteObjectFileRequest{ObjectId: objectID, Filename: filename, Data: data})
	return rpcerrors.FromStatus(err)
}
func (c *ObjectGatewayClient) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	resp, err := c.client.ReadObjectFile(ctx, &sharedv1.ReadObjectFileRequest{ObjectId: objectID, Filename: filename})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return resp.GetData(), nil
}
func (c *ObjectGatewayClient) DeleteFile(ctx context.Context, objectID, filename string) error {
	_, err := c.client.DeleteObjectFile(ctx, &sharedv1.ReadObjectFileRequest{ObjectId: objectID, Filename: filename})
	return rpcerrors.FromStatus(err)
}
func (c *ObjectGatewayClient) ListFiles(ctx context.Context, objectID string) ([]string, error) {
	resp, err := c.client.ListObjectFiles(ctx, &sharedv1.ListObjectFilesRequest{ObjectId: objectID})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return resp.GetFilenames(), nil
}
func (c *ObjectGatewayClient) Reconcile(ctx context.Context) error {
	_, err := c.client.ReconcileObjects(ctx, &sharedv1.ReconcileObjectsRequest{})
	return rpcerrors.FromStatus(err)
}

func (c *TaskStoreClient) CreateTask(ctx context.Context, task *model.Task) error {
	resp, err := c.client.CreateTask(ctx, &sharedv1.TaskRequest{Task: pbconv.TaskToProto(task)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.TaskFromProto(resp.GetTask())
	if convErr == nil {
		*task = *converted
	}
	return convErr
}
func (c *TaskStoreClient) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	resp, err := c.client.GetTask(ctx, &sharedv1.GetTaskRequest{TaskId: taskID})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return pbconv.TaskFromProto(resp.GetTask())
}
func (c *TaskStoreClient) ListTasks(ctx context.Context, filters ...store.TaskFilter) ([]model.Task, error) {
	resp, err := c.client.ListTasks(ctx, &sharedv1.ListTasksRequest{Filter: pbconv.TaskFilterToProto(filters)})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	out := make([]model.Task, 0, len(resp.GetTasks()))
	for _, task := range resp.GetTasks() {
		converted, convErr := pbconv.TaskFromProto(task)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, *converted)
	}
	return out, nil
}
func (c *TaskStoreClient) UpdateTask(ctx context.Context, task *model.Task) error {
	resp, err := c.client.UpdateTask(ctx, &sharedv1.TaskRequest{Task: pbconv.TaskToProto(task)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.TaskFromProto(resp.GetTask())
	if convErr == nil {
		*task = *converted
	}
	return convErr
}
func (c *TaskStoreClient) DeleteTask(ctx context.Context, taskID string) error {
	_, err := c.client.DeleteTask(ctx, &sharedv1.DeleteTaskRequest{TaskId: taskID})
	return rpcerrors.FromStatus(err)
}
func (c *TaskStoreClient) UpsertTask(ctx context.Context, task *model.Task) error {
	resp, err := c.client.UpsertTask(ctx, &sharedv1.TaskRequest{Task: pbconv.TaskToProto(task)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.TaskFromProto(resp.GetTask())
	if convErr == nil {
		*task = *converted
	}
	return convErr
}

func (c *ObservationStoreClient) CreateObservation(ctx context.Context, observation *model.Observation) error {
	resp, err := c.client.CreateObservation(ctx, &sharedv1.ObservationRequest{Observation: pbconv.ObservationToProto(observation)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.ObservationFromProto(resp.GetObservation())
	if convErr == nil {
		*observation = *converted
	}
	return convErr
}
func (c *ObservationStoreClient) GetObservation(ctx context.Context, observationID string) (*model.Observation, error) {
	resp, err := c.client.GetObservation(ctx, &sharedv1.GetObservationRequest{ObservationId: observationID})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return pbconv.ObservationFromProto(resp.GetObservation())
}
func (c *ObservationStoreClient) ListObservations(ctx context.Context, filters ...store.ObservationFilter) ([]model.Observation, error) {
	resp, err := c.client.ListObservations(ctx, &sharedv1.ListObservationsRequest{Filter: pbconv.ObservationFilterToProto(filters)})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	out := make([]model.Observation, 0, len(resp.GetObservations()))
	for _, observation := range resp.GetObservations() {
		converted, convErr := pbconv.ObservationFromProto(observation)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, *converted)
	}
	return out, nil
}
func (c *ObservationStoreClient) UpdateObservation(ctx context.Context, observation *model.Observation) error {
	resp, err := c.client.UpdateObservation(ctx, &sharedv1.ObservationRequest{Observation: pbconv.ObservationToProto(observation)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.ObservationFromProto(resp.GetObservation())
	if convErr == nil {
		*observation = *converted
	}
	return convErr
}
func (c *ObservationStoreClient) DeleteObservation(ctx context.Context, observationID string) error {
	_, err := c.client.DeleteObservation(ctx, &sharedv1.DeleteObservationRequest{ObservationId: observationID})
	return rpcerrors.FromStatus(err)
}
func (c *ObservationStoreClient) UpsertObservation(ctx context.Context, observation *model.Observation) error {
	resp, err := c.client.UpsertObservation(ctx, &sharedv1.ObservationRequest{Observation: pbconv.ObservationToProto(observation)})
	if err != nil {
		return rpcerrors.FromStatus(err)
	}
	converted, convErr := pbconv.ObservationFromProto(resp.GetObservation())
	if convErr == nil {
		*observation = *converted
	}
	return convErr
}

func (c *IdempotencyStoreClient) TryBegin(ctx context.Context, scope, key, resourceID string) (record store.IdempotencyRecord, claimed bool, err error) {
	resp, err := c.client.ClaimIdempotency(ctx, &sharedv1.ClaimIdempotencyRequest{Scope: scope, Key: key, ResourceId: resourceID})
	if err != nil {
		return record, false, rpcerrors.FromStatus(err)
	}
	if resp.GetRecord() != nil {
		record = store.IdempotencyRecord{ResourceID: resp.GetRecord().GetResourceId(), Status: store.IdempotencyStatus(resp.GetRecord().GetStatus())}
	}
	return record, resp.GetClaimed(), nil
}
func (c *IdempotencyStoreClient) MarkCompleted(ctx context.Context, scope, key string) error {
	_, err := c.client.MarkIdempotencyCompleted(ctx, &sharedv1.IdempotencyKeyRequest{Scope: scope, Key: key})
	return rpcerrors.FromStatus(err)
}
func (c *IdempotencyStoreClient) MarkFailed(ctx context.Context, scope, key string) error {
	_, err := c.client.MarkIdempotencyFailed(ctx, &sharedv1.IdempotencyKeyRequest{Scope: scope, Key: key})
	return rpcerrors.FromStatus(err)
}

func (NopObjectStorageStore) CreateObjectFolder(string) error { return fmt.Errorf("not used") }
func (NopObjectStorageStore) ObjectFolderExists(string) (bool, error) {
	return false, fmt.Errorf("not used")
}
func (NopObjectStorageStore) ListObjectFolders() ([]string, error) {
	return nil, fmt.Errorf("not used")
}
func (NopObjectStorageStore) DeleteObjectFolder(string) error { return fmt.Errorf("not used") }
func (NopObjectStorageStore) WriteObjectFile(string, string, []byte) error {
	return fmt.Errorf("not used")
}
func (NopObjectStorageStore) AppendObjectFile(string, string, []byte) error {
	return fmt.Errorf("not used")
}
func (NopObjectStorageStore) ReadObjectFile(objectID, filename string) ([]byte, error) {
	return nil, fmt.Errorf("not used: %s/%s", objectID, filename)
}
func (NopObjectStorageStore) DeleteObjectFile(string, string) error { return fmt.Errorf("not used") }
func (NopObjectStorageStore) ListObjectFolderFiles(string) ([]string, error) {
	return nil, fmt.Errorf("not used")
}
func (NopObjectStorageStore) GetObjectFileInfo(string, string) (model.ObjectFileInfo, error) {
	return model.ObjectFileInfo{}, fmt.Errorf("not used")
}
func (NopObjectStorageStore) ReadManifestFile(string) ([]byte, error) {
	return nil, fmt.Errorf("not used")
}
func (NopObjectStorageStore) WriteManifestFile(string, []byte) error { return fmt.Errorf("not used") }
func (NopObjectStorageStore) ValidateSafeObjectPath(string, string) error {
	return fmt.Errorf("not used")
}
func (NopObjectStorageStore) ReaderForObjectFile(objectID, filename string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not used: %s/%s", objectID, filename)
}
