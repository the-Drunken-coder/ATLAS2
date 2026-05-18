package datastorageclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/anomalyco/atlas-core/services/functions/internal/gateway"
	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"github.com/anomalyco/atlas-core/services/shared/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

var _ gateway.ObjectGateway = (*ObjectGatewayClient)(nil)
var _ gateway.StreamingObjectGateway = (*ObjectGatewayClient)(nil)
var _ store.EntityStore = (*EntityStoreClient)(nil)
var _ store.TaskStore = (*TaskStoreClient)(nil)
var _ store.ObservationStore = (*ObservationStoreClient)(nil)
var _ store.IdempotencyStore = (*IdempotencyStoreClient)(nil)

const defaultReadObjectChunkSize = 64 * 1024
const defaultWriteObjectChunkSize = 64 * 1024

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
		return nil, normalizeStreamingRPCError(err)
	}
	return pbconv.EntityFromProto(resp.GetEntity())
}
func (c *EntityStoreClient) ListEntities(ctx context.Context, params store.EntityListParams) (store.EntityListResult, error) {
	resp, err := c.client.ListEntities(ctx, &sharedv1.ListEntitiesRequest{
		Filter:    pbconv.EntityFilterToProto(params.Filters),
		PageSize:  params.PageSize,
		PageToken: params.PageToken,
	})
	if err != nil {
		return store.EntityListResult{}, rpcerrors.FromStatus(err)
	}
	out := store.EntityListResult{NextPageToken: resp.GetNextPageToken()}
	for _, entity := range resp.GetEntities() {
		converted, convErr := pbconv.EntityFromProto(entity)
		if convErr != nil {
			return store.EntityListResult{}, convErr
		}
		out.Entities = append(out.Entities, *converted)
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
func (c *ObjectGatewayClient) ListObjects(ctx context.Context, params store.ObjectListParams) (store.ObjectListResult, error) {
	resp, err := c.client.ListObjects(ctx, &sharedv1.ListObjectsRequest{
		Filter:    pbconv.ObjectFilterToProto(params.Filters),
		PageSize:  params.PageSize,
		PageToken: params.PageToken,
	})
	if err != nil {
		return store.ObjectListResult{}, rpcerrors.FromStatus(err)
	}
	out := store.ObjectListResult{NextPageToken: resp.GetNextPageToken()}
	for _, object := range resp.GetObjects() {
		converted, convErr := pbconv.ObjectFromProto(object)
		if convErr != nil {
			return store.ObjectListResult{}, convErr
		}
		out.Objects = append(out.Objects, *converted)
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
		return nil, normalizeStreamingRPCError(err)
	}
	return pbconv.ManifestFromProto(resp.GetManifest())
}
func (c *ObjectGatewayClient) WriteFile(ctx context.Context, objectID, filename string, data []byte) (gateway.ManifestResult, error) {
	return writeObjectFile(ctx, c.client, objectID, filename, data)
}
func (c *ObjectGatewayClient) AppendFile(ctx context.Context, objectID, filename string, data []byte) (gateway.ManifestResult, error) {
	return appendObjectFile(ctx, c.client, objectID, filename, data)
}
func (c *ObjectGatewayClient) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	return readObjectFile(ctx, c.client, objectID, filename)
}

func (c *ObjectGatewayClient) OpenWriteFileStream(ctx context.Context, objectID, filename string, expectedSize int64) (gateway.ObjectFileUploadStream, error) {
	stream, err := c.client.WriteObjectFile(ctx)
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return &objectFileUploadStream{
		writeStream: &writeObjectFileUploadStream{
			stream: stream,
			base:   sharedv1.WriteFileChunk{ObjectId: objectID, Filename: filename, ExpectedSize: expectedSize},
		},
	}, nil
}

func (c *ObjectGatewayClient) OpenAppendFileStream(ctx context.Context, objectID, filename string, currentExpectedSize, expectedSize int64) (gateway.ObjectFileUploadStream, error) {
	stream, err := c.client.AppendObjectFile(ctx)
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return &objectFileUploadStream{
		appendStream: &appendObjectFileUploadStream{
			stream: stream,
			base: sharedv1.AppendFileChunk{
				ObjectId:            objectID,
				Filename:            filename,
				ExpectedSize:        expectedSize,
				CurrentExpectedSize: currentExpectedSize,
			},
		},
	}, nil
}

func (c *ObjectGatewayClient) OpenReadFileStream(ctx context.Context, objectID, filename string, chunkSize int64) (gateway.ObjectFileDownloadStream, error) {
	stream, err := c.client.ReadObjectFile(ctx, &sharedv1.ReadFileRequest{
		ObjectId:  objectID,
		Filename:  filename,
		ChunkSize: chunkSize,
	})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return &objectFileDownloadStream{stream: stream}, nil
}
func (c *ObjectGatewayClient) DeleteFile(ctx context.Context, objectID, filename string) (gateway.ManifestResult, error) {
	resp, err := c.client.DeleteObjectFile(ctx, &sharedv1.ReadFileRequest{ObjectId: objectID, Filename: filename})
	if err != nil {
		return gateway.ManifestResult{}, rpcerrors.FromStatus(err)
	}
	return manifestResultFromProto(resp)
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
func (c *TaskStoreClient) ListTasks(ctx context.Context, params store.TaskListParams) (store.TaskListResult, error) {
	resp, err := c.client.ListTasks(ctx, &sharedv1.ListTasksRequest{
		Filter:    pbconv.TaskFilterToProto(params.Filters),
		PageSize:  params.PageSize,
		PageToken: params.PageToken,
	})
	if err != nil {
		return store.TaskListResult{}, rpcerrors.FromStatus(err)
	}
	out := store.TaskListResult{NextPageToken: resp.GetNextPageToken()}
	for _, task := range resp.GetTasks() {
		converted, convErr := pbconv.TaskFromProto(task)
		if convErr != nil {
			return store.TaskListResult{}, convErr
		}
		out.Tasks = append(out.Tasks, *converted)
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
func (c *ObservationStoreClient) ListObservations(ctx context.Context, params store.ObservationListParams) (store.ObservationListResult, error) {
	resp, err := c.client.ListObservations(ctx, &sharedv1.ListObservationsRequest{
		Filter:    pbconv.ObservationFilterToProto(params.Filters),
		PageSize:  params.PageSize,
		PageToken: params.PageToken,
	})
	if err != nil {
		return store.ObservationListResult{}, rpcerrors.FromStatus(err)
	}
	out := store.ObservationListResult{NextPageToken: resp.GetNextPageToken()}
	for _, observation := range resp.GetObservations() {
		converted, convErr := pbconv.ObservationFromProto(observation)
		if convErr != nil {
			return store.ObservationListResult{}, convErr
		}
		out.Observations = append(out.Observations, *converted)
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

func writeObjectFile(ctx context.Context, client datastoragev1.DataStorageServiceClient, objectID, filename string, data []byte) (gateway.ManifestResult, error) {
	stream, err := client.WriteObjectFile(ctx)
	if err != nil {
		return gateway.ManifestResult{}, rpcerrors.FromStatus(err)
	}
	if err := sendWriteObjectChunks(&writeObjectFileUploadStream{
		stream: stream,
		base:   sharedv1.WriteFileChunk{ObjectId: objectID, Filename: filename, ExpectedSize: int64(len(data))},
	}, data); err != nil {
		return gateway.ManifestResult{}, rpcerrors.FromStatus(err)
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return gateway.ManifestResult{}, rpcerrors.FromStatus(err)
	}
	return manifestResultFromProto(resp)
}

func appendObjectFile(ctx context.Context, client datastoragev1.DataStorageServiceClient, objectID, filename string, data []byte) (gateway.ManifestResult, error) {
	info, err := getObjectFileInfo(ctx, client, objectID, filename)
	if err != nil {
		return gateway.ManifestResult{}, err
	}
	currentSize := int64(0)
	if info != nil {
		currentSize = info.GetSize()
	}
	stream, err := client.AppendObjectFile(ctx)
	if err != nil {
		return gateway.ManifestResult{}, rpcerrors.FromStatus(err)
	}
	if err := sendAppendObjectChunks(&appendObjectFileUploadStream{
		stream: stream,
		base: sharedv1.AppendFileChunk{
			ObjectId:            objectID,
			Filename:            filename,
			ExpectedSize:        currentSize + int64(len(data)),
			CurrentExpectedSize: currentSize,
		},
	}, data); err != nil {
		return gateway.ManifestResult{}, rpcerrors.FromStatus(err)
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return gateway.ManifestResult{}, rpcerrors.FromStatus(err)
	}
	return manifestResultFromProto(resp)
}

func readObjectFile(ctx context.Context, client datastoragev1.DataStorageServiceClient, objectID, filename string) ([]byte, error) {
	stream, err := client.ReadObjectFile(ctx, &sharedv1.ReadFileRequest{
		ObjectId:  objectID,
		Filename:  filename,
		ChunkSize: defaultReadObjectChunkSize,
	})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	var (
		buffer       bytes.Buffer
		expectedSize int64 = -1
		sawFinal     bool
	)
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			if !sawFinal {
				return nil, fmt.Errorf("object file stream ended before final chunk")
			}
			if expectedSize >= 0 && int64(buffer.Len()) != expectedSize {
				return nil, fmt.Errorf("object file stream size mismatch: got %d, expected %d", buffer.Len(), expectedSize)
			}
			return buffer.Bytes(), nil
		}
		if err != nil {
			return nil, rpcerrors.FromStatus(err)
		}
		if sawFinal {
			return nil, fmt.Errorf("received chunk after final chunk")
		}
		if expectedSize < 0 {
			expectedSize = chunk.GetTotalSize()
		}
		if _, err := buffer.Write(chunk.GetData()); err != nil {
			return nil, err
		}
		sawFinal = chunk.GetFinalChunk()
	}
}

func getObjectFileInfo(ctx context.Context, client datastoragev1.DataStorageServiceClient, objectID, filename string) (*sharedv1.ObjectFileInfo, error) {
	resp, err := client.GetObjectManifest(ctx, &sharedv1.GetObjectManifestRequest{ObjectId: objectID})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	info, ok := resp.GetManifest().GetFiles()[filename]
	if !ok {
		return nil, nil
	}
	return info, nil
}

type objectFileUploadStream struct {
	writeStream  *writeObjectFileUploadStream
	appendStream *appendObjectFileUploadStream
}

func (s *objectFileUploadStream) SendChunk(data []byte, finalChunk bool) error {
	switch {
	case s.writeStream != nil:
		return s.writeStream.SendChunk(data, finalChunk)
	case s.appendStream != nil:
		return s.appendStream.SendChunk(data, finalChunk)
	default:
		return fmt.Errorf("upload stream is not initialized")
	}
}

func (s *objectFileUploadStream) CloseAndRecv() (gateway.ManifestResult, error) {
	var (
		resp *sharedv1.ObjectManifestResponse
		err  error
	)
	switch {
	case s.writeStream != nil:
		resp, err = s.writeStream.stream.CloseAndRecv()
	case s.appendStream != nil:
		resp, err = s.appendStream.stream.CloseAndRecv()
	default:
		return gateway.ManifestResult{}, fmt.Errorf("upload stream is not initialized")
	}
	if err != nil {
		return gateway.ManifestResult{}, normalizeStreamingRPCError(err)
	}
	return manifestResultFromProto(resp)
}

func (s *objectFileUploadStream) CloseSend() error {
	switch {
	case s.writeStream != nil:
		return s.writeStream.stream.CloseSend()
	case s.appendStream != nil:
		return s.appendStream.stream.CloseSend()
	default:
		return nil
	}
}

type writeObjectFileUploadStream struct {
	stream datastoragev1.DataStorageService_WriteObjectFileClient
	base   sharedv1.WriteFileChunk
}

func (s *writeObjectFileUploadStream) SendChunk(data []byte, finalChunk bool) error {
	chunk := s.base
	chunk.Data = data
	chunk.FinalChunk = finalChunk
	return s.stream.Send(&chunk)
}

type appendObjectFileUploadStream struct {
	stream datastoragev1.DataStorageService_AppendObjectFileClient
	base   sharedv1.AppendFileChunk
}

func (s *appendObjectFileUploadStream) SendChunk(data []byte, finalChunk bool) error {
	chunk := s.base
	chunk.Data = data
	chunk.FinalChunk = finalChunk
	return s.stream.Send(&chunk)
}

type objectFileDownloadStream struct {
	stream datastoragev1.DataStorageService_ReadObjectFileClient
}

func manifestResultFromProto(resp *sharedv1.ObjectManifestResponse) (gateway.ManifestResult, error) {
	manifest, err := pbconv.ManifestFromProto(resp.GetManifest())
	if err != nil {
		return gateway.ManifestResult{}, err
	}
	result := gateway.ManifestResult{
		Manifest:          manifest,
		ManifestCurrent:   true,
		ManifestSyncError: resp.GetManifestSyncError(),
	}
	if resp.GetManifestSyncError() != "" {
		result.ManifestCurrent = false
	}
	if resp.GetManifestCurrent() {
		result.ManifestCurrent = true
	}
	return result, nil
}

func (s *objectFileDownloadStream) RecvChunk() ([]byte, bool, int64, error) {
	chunk, err := s.stream.Recv()
	if err != nil {
		return nil, false, 0, err
	}
	return chunk.GetData(), chunk.GetFinalChunk(), chunk.GetTotalSize(), nil
}

func sendWriteObjectChunks(stream *writeObjectFileUploadStream, data []byte) error {
	return sendUploadChunks(len(data), func(part []byte, finalChunk bool) error {
		return stream.SendChunk(part, finalChunk)
	}, data)
}

func sendAppendObjectChunks(stream *appendObjectFileUploadStream, data []byte) error {
	return sendUploadChunks(len(data), func(part []byte, finalChunk bool) error {
		return stream.SendChunk(part, finalChunk)
	}, data)
}

func sendUploadChunks(total int, send func([]byte, bool) error, data []byte) error {
	if total == 0 {
		return send(nil, true)
	}
	for offset := 0; offset < total; offset += defaultWriteObjectChunkSize {
		end := offset + defaultWriteObjectChunkSize
		if end > total {
			end = total
		}
		if err := send(data[offset:end], end == total); err != nil {
			return err
		}
	}
	return nil
}

func (NopObjectStorageStore) CreateObjectFolder(objectID string) error {
	return nopObjectStorageError("create object folder", objectID, "")
}
func (NopObjectStorageStore) ObjectFolderExists(objectID string) (bool, error) {
	return false, nopObjectStorageError("check object folder", objectID, "")
}
func (NopObjectStorageStore) ListObjectFolders() ([]string, error) {
	return nil, nopObjectStorageError("list object folders", "", "")
}
func (NopObjectStorageStore) DeleteObjectFolder(objectID string) error {
	return nopObjectStorageError("delete object folder", objectID, "")
}
func (NopObjectStorageStore) WriteObjectFile(objectID, filename string, _ []byte) error {
	return nopObjectStorageError("write object file", objectID, filename)
}
func (NopObjectStorageStore) AppendObjectFile(objectID, filename string, _ []byte) error {
	return nopObjectStorageError("append object file", objectID, filename)
}
func (NopObjectStorageStore) ReadObjectFile(objectID, filename string) ([]byte, error) {
	return nil, nopObjectStorageError("read object file", objectID, filename)
}
func (NopObjectStorageStore) DeleteObjectFile(objectID, filename string) error {
	return nopObjectStorageError("delete object file", objectID, filename)
}
func (NopObjectStorageStore) ListObjectFolderFiles(objectID string) ([]string, error) {
	return nil, nopObjectStorageError("list object files", objectID, "")
}
func (NopObjectStorageStore) GetObjectFileInfo(objectID, filename string) (model.ObjectFileInfo, error) {
	return model.ObjectFileInfo{}, nopObjectStorageError("stat object file", objectID, filename)
}
func (NopObjectStorageStore) ReadManifestFile(objectID string) ([]byte, error) {
	return nil, nopObjectStorageError("read manifest", objectID, "")
}
func (NopObjectStorageStore) WriteManifestFile(objectID string, _ []byte) error {
	return nopObjectStorageError("write manifest", objectID, "")
}
func (NopObjectStorageStore) ValidateSafeObjectPath(objectID, filename string) error {
	return nopObjectStorageError("validate object path", objectID, filename)
}
func (NopObjectStorageStore) ReaderForObjectFile(objectID, filename string) (io.ReadCloser, error) {
	return nil, nopObjectStorageError("open object file reader", objectID, filename)
}

func nopObjectStorageError(operation, objectID, filename string) error {
	if objectID != "" && filename != "" {
		return fmt.Errorf("object storage operation %q requires datastorage gRPC client (%s/%s)", operation, objectID, filename)
	}
	if objectID != "" {
		return fmt.Errorf("object storage operation %q requires datastorage gRPC client (%s)", operation, objectID)
	}
	return fmt.Errorf("object storage operation %q requires datastorage gRPC client", operation)
}

func normalizeStreamingRPCError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.NotFound, codes.AlreadyExists, codes.Aborted, codes.InvalidArgument, codes.Canceled, codes.DeadlineExceeded:
		return rpcerrors.FromStatus(err)
	default:
		return err
	}
}
