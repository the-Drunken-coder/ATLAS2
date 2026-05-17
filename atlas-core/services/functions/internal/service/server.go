package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/anomalyco/atlas-core/services/functions/internal/changefeed"
	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"github.com/anomalyco/atlas-core/services/shared/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const MAX_OBJECT_FILE_CHUNK_BYTES = 4*1024*1024 - 4096 // 4 MiB − 4 KiB

type Server struct {
	functionsv1.UnimplementedAtlasFunctionsServiceServer
	functionsv1.UnimplementedChangefeedServiceServer
	funcs functionpkg.Functions
	hub   *changefeed.Hub
}

func NewServer(funcs functionpkg.Functions, hub *changefeed.Hub) *Server {
	if hub == nil {
		hub = changefeed.NewHub()
	}
	return &Server{funcs: funcs, hub: hub}
}

func RegisterGRPC(server grpc.ServiceRegistrar, funcs functionpkg.Functions, hub *changefeed.Hub) {
	handler := NewServer(funcs, hub)
	functionsv1.RegisterAtlasFunctionsServiceServer(server, handler)
	functionsv1.RegisterChangefeedServiceServer(server, handler)
}

func defaultEntityRequestTimestamps(entity *sharedv1.Entity) *sharedv1.Entity {
	if entity == nil || (entity.GetCreatedAt() != nil && entity.GetUpdatedAt() != nil) {
		return entity
	}
	copy := *entity
	now := timestamppb.Now()
	if copy.CreatedAt == nil {
		copy.CreatedAt = now
	}
	if copy.UpdatedAt == nil {
		copy.UpdatedAt = now
	}
	return &copy
}

func defaultObjectRequestTimestamps(object *sharedv1.Object) *sharedv1.Object {
	if object == nil || (object.GetCreatedAt() != nil && object.GetUpdatedAt() != nil) {
		return object
	}
	copy := *object
	now := timestamppb.Now()
	if copy.CreatedAt == nil {
		copy.CreatedAt = now
	}
	if copy.UpdatedAt == nil {
		copy.UpdatedAt = now
	}
	return &copy
}

func defaultTaskRequestTimestamps(task *sharedv1.Task) *sharedv1.Task {
	if task == nil || (task.GetCreatedAt() != nil && task.GetUpdatedAt() != nil) {
		return task
	}
	copy := *task
	now := timestamppb.Now()
	if copy.CreatedAt == nil {
		copy.CreatedAt = now
	}
	if copy.UpdatedAt == nil {
		copy.UpdatedAt = now
	}
	return &copy
}

func defaultObservationRequestTimestamps(observation *sharedv1.Observation) *sharedv1.Observation {
	if observation == nil || (observation.GetCreatedAt() != nil && observation.GetUpdatedAt() != nil) {
		return observation
	}
	copy := *observation
	now := timestamppb.Now()
	if copy.CreatedAt == nil {
		copy.CreatedAt = now
	}
	if copy.UpdatedAt == nil {
		copy.UpdatedAt = now
	}
	return &copy
}

func (s *Server) CreateEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(defaultEntityRequestTimestamps(req.GetEntity()))
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Entity.CreateEntity(ctx, entity); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
func (s *Server) GetEntity(ctx context.Context, req *sharedv1.GetEntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := s.funcs.Entity.GetEntity(ctx, req.GetEntityId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
func (s *Server) ListEntities(ctx context.Context, req *sharedv1.ListEntitiesRequest) (*sharedv1.ListEntitiesResponse, error) {
	filters, err := pbconv.EntityFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	result, err := s.funcs.Entity.ListEntities(ctx, store.EntityListParams{
		Filters:   filters,
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListEntitiesResponse{NextPageToken: result.NextPageToken}
	for i := range result.Entities {
		resp.Entities = append(resp.Entities, pbconv.EntityToProto(&result.Entities[i]))
	}
	return resp, nil
}
func (s *Server) UpdateEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(req.GetEntity())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Entity.UpdateEntity(ctx, entity); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
func (s *Server) DeleteEntity(ctx context.Context, req *sharedv1.DeleteEntityRequest) (*emptypb.Empty, error) {
	if err := s.funcs.Entity.DeleteEntity(ctx, req.GetEntityId()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *Server) UpsertEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(defaultEntityRequestTimestamps(req.GetEntity()))
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Entity.UpsertEntity(ctx, entity); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}

func (s *Server) CreateObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(defaultObjectRequestTimestamps(req.GetObject()))
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	var opts []functionpkg.IdempotencyOption
	if req.IdempotencyKey != nil && req.GetIdempotencyKey() != "" {
		opts = append(opts, functionpkg.WithIdempotencyKey(req.GetIdempotencyKey()))
	}
	if err := s.funcs.Object.CreateObject(ctx, object, opts...); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *Server) GetObject(ctx context.Context, req *sharedv1.GetObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := s.funcs.Object.GetObject(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *Server) ListObjects(ctx context.Context, req *sharedv1.ListObjectsRequest) (*sharedv1.ListObjectsResponse, error) {
	filters, err := pbconv.ObjectFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	result, err := s.funcs.Object.ListObjects(ctx, store.ObjectListParams{
		Filters:   filters,
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListObjectsResponse{NextPageToken: result.NextPageToken}
	for i := range result.Objects {
		resp.Objects = append(resp.Objects, pbconv.ObjectToProto(&result.Objects[i]))
	}
	return resp, nil
}
func (s *Server) UpdateObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(req.GetObject())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Object.UpdateObject(ctx, object); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *Server) DeleteObject(ctx context.Context, req *sharedv1.DeleteObjectRequest) (*emptypb.Empty, error) {
	if err := s.funcs.Object.DeleteObject(ctx, req.GetObjectId()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *Server) UpsertObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(defaultObjectRequestTimestamps(req.GetObject()))
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Object.UpsertObject(ctx, object); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *Server) GetObjectManifest(ctx context.Context, req *sharedv1.GetObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	manifest, err := s.funcs.Object.GetObjectManifest(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest), ManifestCurrent: true}, nil
}
func (s *Server) UpdateObjectManifest(ctx context.Context, req *sharedv1.UpdateObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	manifest, err := pbconv.ManifestFromProto(req.GetManifest())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Object.UpdateObjectManifest(ctx, req.GetObjectId(), manifest); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest), ManifestCurrent: true}, nil
}
func (s *Server) WriteObjectFile(stream functionsv1.AtlasFunctionsService_WriteObjectFileServer) error {
	gateway, ok := s.funcs.Object.StreamingGateway()
	if !ok {
		return status.Error(codes.Internal, "streaming object gateway is not configured")
	}
	firstChunk, metadata, err := receiveFirstWriteChunk(stream)
	if err != nil {
		return err
	}
	upload, err := gateway.OpenWriteFileStream(stream.Context(), metadata.objectID, metadata.filename, metadata.expectedSize)
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	result, err := forwardWriteChunks(stream, upload, metadata, firstChunk.GetData(), firstChunk.GetFinalChunk(), MAX_OBJECT_FILE_CHUNK_BYTES)
	if err != nil {
		if closeErr := upload.CloseSend(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := s.funcs.Object.PublishObjectUpdated(stream.Context(), metadata.objectID); err != nil {
		return rpcerrors.ToStatus(err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{
		Manifest:          pbconv.ManifestToProto(result.Manifest),
		ManifestCurrent:   result.ManifestCurrent,
		ManifestSyncError: result.ManifestSyncError,
	})
}
func (s *Server) AppendObjectFile(stream functionsv1.AtlasFunctionsService_AppendObjectFileServer) error {
	gateway, ok := s.funcs.Object.StreamingGateway()
	if !ok {
		return status.Error(codes.Internal, "streaming object gateway is not configured")
	}
	firstChunk, metadata, err := receiveFirstAppendChunk(stream)
	if err != nil {
		return err
	}
	upload, err := gateway.OpenAppendFileStream(
		stream.Context(),
		metadata.objectID,
		metadata.filename,
		metadata.currentExpectedSize,
		metadata.expectedSize,
	)
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	result, err := forwardAppendChunks(stream, upload, metadata, firstChunk.GetData(), firstChunk.GetFinalChunk(), MAX_OBJECT_FILE_CHUNK_BYTES)
	if err != nil {
		if closeErr := upload.CloseSend(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := s.funcs.Object.PublishObjectUpdated(stream.Context(), metadata.objectID); err != nil {
		return rpcerrors.ToStatus(err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{
		Manifest:          pbconv.ManifestToProto(result.Manifest),
		ManifestCurrent:   result.ManifestCurrent,
		ManifestSyncError: result.ManifestSyncError,
	})
}
func (s *Server) ReadObjectFile(req *sharedv1.ReadFileRequest, stream functionsv1.AtlasFunctionsService_ReadObjectFileServer) error {
	gateway, ok := s.funcs.Object.StreamingGateway()
	if !ok {
		return status.Error(codes.Internal, "streaming object gateway is not configured")
	}
	download, err := gateway.OpenReadFileStream(stream.Context(), req.GetObjectId(), req.GetFilename(), req.GetChunkSize())
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	return proxyReadChunks(download, stream.Send)
}
func (s *Server) DeleteObjectFile(ctx context.Context, req *sharedv1.ReadFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	result, err := s.funcs.Object.DeleteFile(ctx, req.GetObjectId(), req.GetFilename())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{
		Manifest:          pbconv.ManifestToProto(result.Manifest),
		ManifestCurrent:   result.ManifestCurrent,
		ManifestSyncError: result.ManifestSyncError,
	}, nil
}
func (s *Server) ListObjectFiles(ctx context.Context, req *sharedv1.ListObjectFilesRequest) (*sharedv1.ListObjectFilesResponse, error) {
	files, err := s.funcs.Object.ListFiles(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ListObjectFilesResponse{Filenames: files}, nil
}

func (s *Server) CreateTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := pbconv.TaskFromProto(defaultTaskRequestTimestamps(req.GetTask()))
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	var opts []functionpkg.IdempotencyOption
	if req.IdempotencyKey != nil && req.GetIdempotencyKey() != "" {
		opts = append(opts, functionpkg.WithIdempotencyKey(req.GetIdempotencyKey()))
	}
	if err := s.funcs.Task.CreateTask(ctx, task, opts...); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
func (s *Server) GetTask(ctx context.Context, req *sharedv1.GetTaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := s.funcs.Task.GetTask(ctx, req.GetTaskId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
func (s *Server) ListTasks(ctx context.Context, req *sharedv1.ListTasksRequest) (*sharedv1.ListTasksResponse, error) {
	filters, err := pbconv.TaskFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	result, err := s.funcs.Task.ListTasks(ctx, store.TaskListParams{
		Filters:   filters,
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListTasksResponse{NextPageToken: result.NextPageToken}
	for i := range result.Tasks {
		resp.Tasks = append(resp.Tasks, pbconv.TaskToProto(&result.Tasks[i]))
	}
	return resp, nil
}
func (s *Server) UpdateTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := pbconv.TaskFromProto(req.GetTask())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Task.UpdateTask(ctx, task); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
func (s *Server) DeleteTask(ctx context.Context, req *sharedv1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if err := s.funcs.Task.DeleteTask(ctx, req.GetTaskId()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *Server) UpsertTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := pbconv.TaskFromProto(defaultTaskRequestTimestamps(req.GetTask()))
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Task.UpsertTask(ctx, task); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}

func (s *Server) CreateObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := pbconv.ObservationFromProto(defaultObservationRequestTimestamps(req.GetObservation()))
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Observation.CreateObservation(ctx, observation); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
func (s *Server) GetObservation(ctx context.Context, req *sharedv1.GetObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := s.funcs.Observation.GetObservation(ctx, req.GetObservationId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
func (s *Server) ListObservations(ctx context.Context, req *sharedv1.ListObservationsRequest) (*sharedv1.ListObservationsResponse, error) {
	filters, err := pbconv.ObservationFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	result, err := s.funcs.Observation.ListObservations(ctx, store.ObservationListParams{
		Filters:   filters,
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListObservationsResponse{NextPageToken: result.NextPageToken}
	for i := range result.Observations {
		resp.Observations = append(resp.Observations, pbconv.ObservationToProto(&result.Observations[i]))
	}
	return resp, nil
}
func (s *Server) UpdateObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := pbconv.ObservationFromProto(req.GetObservation())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Observation.UpdateObservation(ctx, observation); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
func (s *Server) DeleteObservation(ctx context.Context, req *sharedv1.DeleteObservationRequest) (*emptypb.Empty, error) {
	if err := s.funcs.Observation.DeleteObservation(ctx, req.GetObservationId()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *Server) UpsertObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := pbconv.ObservationFromProto(defaultObservationRequestTimestamps(req.GetObservation()))
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Observation.UpsertObservation(ctx, observation); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}

func (s *Server) SubscribeMutations(req *functionsv1.SubscribeMutationsRequest, stream functionsv1.ChangefeedService_SubscribeMutationsServer) error {
	sub := s.hub.Subscribe(stream.Context())
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				if err := sub.Err(); err != nil {
					if errors.Is(err, context.Canceled) {
						return rpcerrors.ToStatus(err)
					}
					return status.Error(codes.ResourceExhausted, err.Error())
				}
				return nil
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		case <-s.hub.Done():
			return rpcerrors.ToStatus(context.Canceled)
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

type receivedWriteFile struct {
	objectID     string
	filename     string
	expectedSize int64
}

type receivedAppendFile struct {
	receivedWriteFile
	currentExpectedSize int64
}

func receiveFirstWriteChunk(stream interface {
	Context() context.Context
	Recv() (*sharedv1.WriteFileChunk, error)
}) (*sharedv1.WriteFileChunk, receivedWriteFile, error) {
	chunk, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return nil, receivedWriteFile{}, status.Error(codes.InvalidArgument, "at least one chunk is required")
	}
	if err != nil {
		return nil, receivedWriteFile{}, err
	}
	file := receivedWriteFile{
		objectID:     chunk.GetObjectId(),
		filename:     chunk.GetFilename(),
		expectedSize: chunk.GetExpectedSize(),
	}
	if err := validateWriteChunkMetadata(file, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
		return nil, receivedWriteFile{}, err
	}
	if file.objectID == "" {
		return nil, receivedWriteFile{}, status.Error(codes.InvalidArgument, "object_id is required")
	}
	if file.filename == "" {
		return nil, receivedWriteFile{}, status.Error(codes.InvalidArgument, "filename is required")
	}
	return chunk, file, nil
}

func receiveFirstAppendChunk(
	stream interface {
		Context() context.Context
		Recv() (*sharedv1.AppendFileChunk, error)
	},
) (*sharedv1.AppendFileChunk, receivedAppendFile, error) {
	chunk, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return nil, receivedAppendFile{}, status.Error(codes.InvalidArgument, "at least one chunk is required")
	}
	if err != nil {
		return nil, receivedAppendFile{}, err
	}
	file := receivedAppendFile{
		receivedWriteFile: receivedWriteFile{
			objectID:     chunk.GetObjectId(),
			filename:     chunk.GetFilename(),
			expectedSize: chunk.GetExpectedSize(),
		},
		currentExpectedSize: chunk.GetCurrentExpectedSize(),
	}
	if err := validateWriteChunkMetadata(file.receivedWriteFile, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
		return nil, receivedAppendFile{}, err
	}
	if file.objectID == "" {
		return nil, receivedAppendFile{}, status.Error(codes.InvalidArgument, "object_id is required")
	}
	if file.filename == "" {
		return nil, receivedAppendFile{}, status.Error(codes.InvalidArgument, "filename is required")
	}
	return chunk, file, nil
}

func validateWriteChunkMetadata(file receivedWriteFile, objectID, filename string, expectedSize int64) error {
	if objectID != file.objectID {
		return status.Error(codes.InvalidArgument, "object_id must match across all chunks")
	}
	if filename != file.filename {
		return status.Error(codes.InvalidArgument, "filename must match across all chunks")
	}
	if expectedSize != 0 && expectedSize != file.expectedSize {
		return status.Error(codes.InvalidArgument, "expected_size must match across all chunks")
	}
	return nil
}

func forwardWriteChunks(
	stream interface {
		Recv() (*sharedv1.WriteFileChunk, error)
	},
	upload functionpkg.ObjectFileUploadStream,
	file receivedWriteFile,
	firstData []byte,
	firstFinalChunk bool,
	maxBytes int64,
) (functionpkg.ManifestResult, error) {
	totalBytes := int64(0)
	next := func(data []byte, finalChunk bool) (functionpkg.ManifestResult, error) {
		if err := validateChunkSize(data, maxBytes); err != nil {
			return functionpkg.ManifestResult{}, err
		}
		totalBytes += int64(len(data))
		if err := upload.SendChunk(data, finalChunk); err != nil {
			return functionpkg.ManifestResult{}, err
		}
		if !finalChunk {
			return functionpkg.ManifestResult{ManifestCurrent: true}, nil
		}
		if file.expectedSize != 0 && totalBytes != file.expectedSize {
			return functionpkg.ManifestResult{}, status.Error(codes.InvalidArgument, fmt.Sprintf("expected_size mismatch: got %d bytes, expected %d", totalBytes, file.expectedSize))
		}
		if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
			if err != nil {
				return functionpkg.ManifestResult{}, err
			}
			return functionpkg.ManifestResult{}, status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		return upload.CloseAndRecv()
	}
	if result, err := next(firstData, firstFinalChunk); result.Manifest != nil || result.ManifestSyncError != "" || err != nil {
		return result, err
	}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return functionpkg.ManifestResult{}, status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
		}
		if err != nil {
			return functionpkg.ManifestResult{}, err
		}
		if err := validateWriteChunkMetadata(file, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
			return functionpkg.ManifestResult{}, err
		}
		if result, err := next(chunk.GetData(), chunk.GetFinalChunk()); result.Manifest != nil || result.ManifestSyncError != "" || err != nil {
			return result, err
		}
	}
}

func forwardAppendChunks(
	stream interface {
		Recv() (*sharedv1.AppendFileChunk, error)
	},
	upload functionpkg.ObjectFileUploadStream,
	file receivedAppendFile,
	firstData []byte,
	firstFinalChunk bool,
	maxBytes int64,
) (functionpkg.ManifestResult, error) {
	totalBytes := int64(0)
	next := func(data []byte, finalChunk bool) (functionpkg.ManifestResult, error) {
		if err := validateChunkSize(data, maxBytes); err != nil {
			return functionpkg.ManifestResult{}, err
		}
		totalBytes += int64(len(data))
		if err := upload.SendChunk(data, finalChunk); err != nil {
			return functionpkg.ManifestResult{}, err
		}
		if !finalChunk {
			return functionpkg.ManifestResult{ManifestCurrent: true}, nil
		}
		if file.expectedSize != 0 && file.currentExpectedSize+totalBytes != file.expectedSize {
			return functionpkg.ManifestResult{}, status.Error(codes.InvalidArgument, fmt.Sprintf(
				"expected_size mismatch: got %d bytes after append, expected %d",
				file.currentExpectedSize+totalBytes, file.expectedSize))
		}
		if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
			if err != nil {
				return functionpkg.ManifestResult{}, err
			}
			return functionpkg.ManifestResult{}, status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		return upload.CloseAndRecv()
	}
	if result, err := next(firstData, firstFinalChunk); result.Manifest != nil || result.ManifestSyncError != "" || err != nil {
		return result, err
	}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return functionpkg.ManifestResult{}, status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
		}
		if err != nil {
			return functionpkg.ManifestResult{}, err
		}
		if err := validateWriteChunkMetadata(file.receivedWriteFile, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
			return functionpkg.ManifestResult{}, err
		}
		if chunk.GetCurrentExpectedSize() != file.currentExpectedSize {
			return functionpkg.ManifestResult{}, status.Error(codes.InvalidArgument, "current_expected_size must match across all chunks")
		}
		if result, err := next(chunk.GetData(), chunk.GetFinalChunk()); result.Manifest != nil || result.ManifestSyncError != "" || err != nil {
			return result, err
		}
	}
}

func validateChunkSize(data []byte, maxBytes int64) error {
	if int64(len(data)) > maxBytes {
		return status.Error(codes.ResourceExhausted, fmt.Sprintf("chunk size %d exceeds maximum of %d bytes", len(data), maxBytes))
	}
	return nil
}

func proxyReadChunks(download functionpkg.ObjectFileDownloadStream, send func(*sharedv1.FileChunk) error) error {
	for {
		data, finalChunk, totalSize, err := download.RecvChunk()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := send(&sharedv1.FileChunk{Data: data, FinalChunk: finalChunk, TotalSize: totalSize}); err != nil {
			return err
		}
		if finalChunk {
			return nil
		}
	}
}
