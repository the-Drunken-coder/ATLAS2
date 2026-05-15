package service

import (
	"context"
	"fmt"

	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// MAX_OBJECT_FILE_BYTES is the maximum allowed file size for unary
// WriteObjectFile and AppendObjectFile RPCs. Files larger than this
// limit must use a chunked streaming API (future work). The value is
// set below gRPC's default 4 MiB message-size ceiling to leave room
// for protobuf framing and gRPC metadata overhead.
const MAX_OBJECT_FILE_BYTES = 4*1024*1024 - 4096 // 4 MiB − 4 KiB

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
	entities, err := s.svc.ListEntities(ctx, pbconv.EntityFiltersFromProto(req.GetFilter())...)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListEntitiesResponse{}
	for i := range entities {
		resp.Entities = append(resp.Entities, pbconv.EntityToProto(&entities[i]))
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
	objects, err := s.svc.ListObjects(ctx, pbconv.ObjectFiltersFromProto(req.GetFilter())...)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListObjectsResponse{}
	for i := range objects {
		resp.Objects = append(resp.Objects, pbconv.ObjectToProto(&objects[i]))
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
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)}, nil
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
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)}, nil
}
func (s *RPCServer) WriteObjectFile(ctx context.Context, req *sharedv1.WriteObjectFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	if len(req.GetData()) > MAX_OBJECT_FILE_BYTES {
		return nil, status.Error(codes.ResourceExhausted, fmt.Sprintf(
			"object file exceeds maximum size of %d bytes (%d provided)",
			MAX_OBJECT_FILE_BYTES, len(req.GetData())))
	}
	result, err := s.svc.WriteObjectFile(ctx, req.GetObjectId(), req.GetFilename(), req.GetData())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{
		Manifest:          pbconv.ManifestToProto(result.Manifest),
		ManifestCurrent:   result.ManifestCurrent,
		ManifestSyncError: result.SyncError,
	}, nil
}
func (s *RPCServer) AppendObjectFile(ctx context.Context, req *sharedv1.WriteObjectFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	if len(req.GetData()) > MAX_OBJECT_FILE_BYTES {
		return nil, status.Error(codes.ResourceExhausted, fmt.Sprintf(
			"object file exceeds maximum size of %d bytes (%d provided)",
			MAX_OBJECT_FILE_BYTES, len(req.GetData())))
	}
	result, err := s.svc.AppendObjectFile(ctx, req.GetObjectId(), req.GetFilename(), req.GetData())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{
		Manifest:          pbconv.ManifestToProto(result.Manifest),
		ManifestCurrent:   result.ManifestCurrent,
		ManifestSyncError: result.SyncError,
	}, nil
}
func (s *RPCServer) ReadObjectFile(ctx context.Context, req *sharedv1.ReadObjectFileRequest) (*sharedv1.ObjectFileContent, error) {
	data, err := s.svc.ReadObjectFile(ctx, req.GetObjectId(), req.GetFilename())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectFileContent{Data: data}, nil
}
func (s *RPCServer) DeleteObjectFile(ctx context.Context, req *sharedv1.ReadObjectFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	result, err := s.svc.DeleteObjectFile(ctx, req.GetObjectId(), req.GetFilename())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{
		Manifest:          pbconv.ManifestToProto(result.Manifest),
		ManifestCurrent:   result.ManifestCurrent,
		ManifestSyncError: result.SyncError,
	}, nil
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
	task, err := pbconv.TaskFromProto(req.GetTask())
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
	tasks, err := s.svc.ListTasks(ctx, pbconv.TaskFiltersFromProto(req.GetFilter())...)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListTasksResponse{}
	for i := range tasks {
		resp.Tasks = append(resp.Tasks, pbconv.TaskToProto(&tasks[i]))
	}
	return resp, nil
}
func (s *RPCServer) UpdateTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := pbconv.TaskFromProto(req.GetTask())
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
	task, err := pbconv.TaskFromProto(req.GetTask())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.svc.UpsertTask(ctx, task); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}

func (s *RPCServer) CreateObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := pbconv.ObservationFromProto(req.GetObservation())
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
	observations, err := s.svc.ListObservations(ctx, pbconv.ObservationFiltersFromProto(req.GetFilter())...)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListObservationsResponse{}
	for i := range observations {
		resp.Observations = append(resp.Observations, pbconv.ObservationToProto(&observations[i]))
	}
	return resp, nil
}
func (s *RPCServer) UpdateObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := pbconv.ObservationFromProto(req.GetObservation())
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
	observation, err := pbconv.ObservationFromProto(req.GetObservation())
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
