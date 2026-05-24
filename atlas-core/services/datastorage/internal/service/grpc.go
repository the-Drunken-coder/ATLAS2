package service

import (
	"context"
	"errors"
	"io"

	"github.com/anomalyco/atlas-core/services/datastorage/internal/objectstorage"
	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/objectstreaming"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"github.com/anomalyco/atlas-core/services/shared/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RPCServer struct {
	datastoragev1.UnimplementedDataStorageServiceServer
	svc *Service
}

func NewRPCServer(svc *Service) *RPCServer {
	return &RPCServer{svc: svc}
}

func RegisterGRPC(server grpc.ServiceRegistrar, svc *Service) {
	datastoragev1.RegisterDataStorageServiceServer(server, NewRPCServer(svc))
}

func (s *RPCServer) CreateEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(req.GetEntity())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.CreateEntity(ctx, entity); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
func (s *RPCServer) GetEntity(ctx context.Context, req *sharedv1.GetEntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := s.svc.GetEntity(ctx, req.GetEntityId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
func (s *RPCServer) ListEntities(ctx context.Context, req *sharedv1.ListEntitiesRequest) (*sharedv1.ListEntitiesResponse, error) {
	filters, err := pbconv.EntityFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	result, err := s.svc.ListEntities(ctx, store.EntityListParams{
		Filters:        filters,
		PageSize:       req.GetPageSize(),
		PageToken:      req.GetPageToken(),
		StrictSnapshot: req.GetStrictSnapshot(),
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
func (s *RPCServer) UpdateEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(req.GetEntity())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.UpdateEntity(ctx, entity); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
func (s *RPCServer) DeleteEntity(ctx context.Context, req *sharedv1.DeleteEntityRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteEntity(ctx, req.GetEntityId()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *RPCServer) UpsertEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(req.GetEntity())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.UpsertEntity(ctx, entity); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}

func (s *RPCServer) CreateObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(req.GetObject())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.CreateObject(ctx, object); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *RPCServer) EnsureObjectCreated(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(req.GetObject())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.EnsureObjectCreated(ctx, object); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *RPCServer) GetObject(ctx context.Context, req *sharedv1.GetObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := s.svc.GetObject(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *RPCServer) ListObjects(ctx context.Context, req *sharedv1.ListObjectsRequest) (*sharedv1.ListObjectsResponse, error) {
	filters, err := pbconv.ObjectFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	result, err := s.svc.ListObjects(ctx, store.ObjectListParams{
		Filters:        filters,
		PageSize:       req.GetPageSize(),
		PageToken:      req.GetPageToken(),
		StrictSnapshot: req.GetStrictSnapshot(),
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
func (s *RPCServer) UpdateObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(req.GetObject())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.UpdateObject(ctx, object); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *RPCServer) DeleteObject(ctx context.Context, req *sharedv1.DeleteObjectRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteObject(ctx, req.GetObjectId()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *RPCServer) UpsertObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(req.GetObject())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.UpsertObject(ctx, object); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *RPCServer) GetObjectManifest(ctx context.Context, req *sharedv1.GetObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	manifest, err := s.svc.GetObjectManifest(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest), ManifestCurrent: true}, nil
}
func (s *RPCServer) UpdateObjectManifest(ctx context.Context, req *sharedv1.UpdateObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	manifest, err := pbconv.ManifestFromProto(req.GetManifest())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	manifest, err = s.svc.UpdateObjectManifest(ctx, req.GetObjectId(), manifest)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest), ManifestCurrent: true}, nil
}
func (s *RPCServer) WriteObjectFile(stream datastoragev1.DataStorageService_WriteObjectFileServer) error {
	firstChunk, file, err := objectstreaming.ReceiveFirstWriteChunk(stream)
	if err != nil {
		return err
	}
	manifest, err := s.svc.StreamWriteObjectFile(stream.Context(), file.ObjectID, file.Filename, func(w io.Writer) error {
		return objectstreaming.ProcessWriteChunks(
			stream.Recv,
			file,
			firstChunk.GetData(),
			firstChunk.GetFinalChunk(),
			objectstreaming.MaxChunkPayloadBytes,
			objectstreaming.NewWriterSink(w, file.ExpectedSize),
		)
	})
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest), ManifestCurrent: true})
}
func (s *RPCServer) AppendObjectFile(stream datastoragev1.DataStorageService_AppendObjectFileServer) error {
	firstChunk, file, err := objectstreaming.ReceiveFirstAppendChunk(stream)
	if err != nil {
		return err
	}
	manifest, err := s.svc.StreamAppendObjectFile(
		stream.Context(),
		file.ObjectID,
		file.Filename,
		file.CurrentExpectedSize,
		func(w io.Writer, currentSize int64) error {
			return objectstreaming.ProcessAppendChunks(
				stream.Recv,
				file,
				firstChunk.GetData(),
				firstChunk.GetFinalChunk(),
				objectstreaming.MaxChunkPayloadBytes,
				objectstreaming.NewAppendWriterSink(w, currentSize, file.ExpectedSize),
			)
		},
	)
	if err != nil {
		var preconditionErr *objectstorage.AppendSizePreconditionError
		if errors.As(err, &preconditionErr) {
			return status.Error(codes.FailedPrecondition, preconditionErr.Error())
		}
		return rpcerrors.ToStatus(err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest), ManifestCurrent: true})
}
func (s *RPCServer) ReadObjectFile(req *sharedv1.ReadFileRequest, stream datastoragev1.DataStorageService_ReadObjectFileServer) error {
	reader, totalSize, err := s.svc.OpenReadObjectFile(stream.Context(), req.GetObjectId(), req.GetFilename())
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	defer func() { _ = reader.Close() }()
	return objectstreaming.SendObjectFileChunks(reader, totalSize, req.GetChunkSize(), stream.Send)
}
func (s *RPCServer) DeleteObjectFile(ctx context.Context, req *sharedv1.ReadFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	manifest, err := s.svc.DeleteObjectFile(ctx, req.GetObjectId(), req.GetFilename())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest), ManifestCurrent: true}, nil
}

func (s *RPCServer) ListObjectFiles(ctx context.Context, req *sharedv1.ListObjectFilesRequest) (*sharedv1.ListObjectFilesResponse, error) {
	files, err := s.svc.ListObjectFiles(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ListObjectFilesResponse{Filenames: files}, nil
}
func (s *RPCServer) ReconcileObjects(ctx context.Context, req *sharedv1.ReconcileObjectsRequest) (*emptypb.Empty, error) {
	if err := s.svc.ReconcileObjects(ctx); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *RPCServer) CreateTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	taskProto := req.GetTask()
	defaultProtoTimestamp(taskProto.GetCreatedAt(), func(ts *timestamppb.Timestamp) { taskProto.CreatedAt = ts })
	defaultProtoTimestamp(taskProto.GetUpdatedAt(), func(ts *timestamppb.Timestamp) { taskProto.UpdatedAt = ts })
	task, err := pbconv.TaskFromProto(taskProto)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.CreateTask(ctx, task); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
func (s *RPCServer) GetTask(ctx context.Context, req *sharedv1.GetTaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := s.svc.GetTask(ctx, req.GetTaskId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
func (s *RPCServer) ListTasks(ctx context.Context, req *sharedv1.ListTasksRequest) (*sharedv1.ListTasksResponse, error) {
	filters, err := pbconv.TaskFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	result, err := s.svc.ListTasks(ctx, store.TaskListParams{
		Filters:        filters,
		PageSize:       req.GetPageSize(),
		PageToken:      req.GetPageToken(),
		StrictSnapshot: req.GetStrictSnapshot(),
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
func (s *RPCServer) UpdateTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	taskProto := req.GetTask()
	defaultProtoTimestamp(taskProto.GetCreatedAt(), func(ts *timestamppb.Timestamp) { taskProto.CreatedAt = ts })
	defaultProtoTimestamp(taskProto.GetUpdatedAt(), func(ts *timestamppb.Timestamp) { taskProto.UpdatedAt = ts })
	task, err := pbconv.TaskFromProto(taskProto)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.UpdateTask(ctx, task); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
func (s *RPCServer) DeleteTask(ctx context.Context, req *sharedv1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteTask(ctx, req.GetTaskId()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *RPCServer) UpsertTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	taskProto := req.GetTask()
	defaultProtoTimestamp(taskProto.GetCreatedAt(), func(ts *timestamppb.Timestamp) { taskProto.CreatedAt = ts })
	defaultProtoTimestamp(taskProto.GetUpdatedAt(), func(ts *timestamppb.Timestamp) { taskProto.UpdatedAt = ts })
	task, err := pbconv.TaskFromProto(taskProto)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.UpsertTask(ctx, task); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}

func (s *RPCServer) CreateObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	obsProto := req.GetObservation()
	defaultProtoTimestamp(obsProto.GetCreatedAt(), func(ts *timestamppb.Timestamp) { obsProto.CreatedAt = ts })
	defaultProtoTimestamp(obsProto.GetUpdatedAt(), func(ts *timestamppb.Timestamp) { obsProto.UpdatedAt = ts })
	observation, err := pbconv.ObservationFromProto(obsProto)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.CreateObservation(ctx, observation); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
func (s *RPCServer) GetObservation(ctx context.Context, req *sharedv1.GetObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := s.svc.GetObservation(ctx, req.GetObservationId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
func (s *RPCServer) ListObservations(ctx context.Context, req *sharedv1.ListObservationsRequest) (*sharedv1.ListObservationsResponse, error) {
	filters, err := pbconv.ObservationFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	result, err := s.svc.ListObservations(ctx, store.ObservationListParams{
		Filters:        filters,
		PageSize:       req.GetPageSize(),
		PageToken:      req.GetPageToken(),
		StrictSnapshot: req.GetStrictSnapshot(),
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
func (s *RPCServer) UpdateObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	obsProto := req.GetObservation()
	defaultProtoTimestamp(obsProto.GetCreatedAt(), func(ts *timestamppb.Timestamp) { obsProto.CreatedAt = ts })
	defaultProtoTimestamp(obsProto.GetUpdatedAt(), func(ts *timestamppb.Timestamp) { obsProto.UpdatedAt = ts })
	observation, err := pbconv.ObservationFromProto(obsProto)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.UpdateObservation(ctx, observation); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
func (s *RPCServer) DeleteObservation(ctx context.Context, req *sharedv1.DeleteObservationRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteObservation(ctx, req.GetObservationId()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *RPCServer) UpsertObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	obsProto := req.GetObservation()
	defaultProtoTimestamp(obsProto.GetCreatedAt(), func(ts *timestamppb.Timestamp) { obsProto.CreatedAt = ts })
	defaultProtoTimestamp(obsProto.GetUpdatedAt(), func(ts *timestamppb.Timestamp) { obsProto.UpdatedAt = ts })
	observation, err := pbconv.ObservationFromProto(obsProto)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.UpsertObservation(ctx, observation); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}

func (s *RPCServer) ClaimIdempotency(ctx context.Context, req *sharedv1.ClaimIdempotencyRequest) (*sharedv1.ClaimIdempotencyResponse, error) {
	record, claimed, err := s.svc.ClaimIdempotency(ctx, req.GetScope(), req.GetKey(), req.GetResourceId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ClaimIdempotencyResponse{Claimed: claimed, Record: &sharedv1.IdempotencyRecord{ResourceId: record.ResourceID, Status: string(record.Status)}}, nil
}
func (s *RPCServer) MarkIdempotencyCompleted(ctx context.Context, req *sharedv1.IdempotencyKeyRequest) (*emptypb.Empty, error) {
	if err := s.svc.MarkIdempotencyCompleted(ctx, req.GetScope(), req.GetKey()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *RPCServer) MarkIdempotencyFailed(ctx context.Context, req *sharedv1.IdempotencyKeyRequest) (*emptypb.Empty, error) {
	if err := s.svc.MarkIdempotencyFailed(ctx, req.GetScope(), req.GetKey()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// defaultProtoTimestamp sets ts to the result of set(now) if ts is nil.
// This ensures server-managed timestamps are populated before proto→model
// conversion so pbconv doesn't reject nil timestamps.
func defaultProtoTimestamp(ts *timestamppb.Timestamp, set func(*timestamppb.Timestamp)) {
	if ts == nil {
		set(timestamppb.Now())
	}
}
