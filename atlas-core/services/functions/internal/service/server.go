package service

import (
	"context"
	"errors"

	"github.com/anomalyco/atlas-core/services/functions/internal/changefeed"
	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	functionsv1.UnimplementedAtlasFunctionsServiceServer
	functionsv1.UnimplementedChangefeedServiceServer
	funcs functionpkg.Functions
	hub   *changefeed.Hub
}

func NewServer(funcs functionpkg.Functions, hub *changefeed.Hub) *Server {
	return &Server{funcs: funcs, hub: hub}
}

func RegisterGRPC(server grpc.ServiceRegistrar, funcs functionpkg.Functions, hub *changefeed.Hub) {
	handler := NewServer(funcs, hub)
	functionsv1.RegisterAtlasFunctionsServiceServer(server, handler)
	functionsv1.RegisterChangefeedServiceServer(server, handler)
}

func (s *Server) CreateEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(req.GetEntity())
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
	entities, err := s.funcs.Entity.ListEntities(ctx, pbconv.EntityFiltersFromProto(req.GetFilter())...)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListEntitiesResponse{}
	for i := range entities {
		resp.Entities = append(resp.Entities, pbconv.EntityToProto(&entities[i]))
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
	entity, err := pbconv.EntityFromProto(req.GetEntity())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Entity.UpsertEntity(ctx, entity); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}

func (s *Server) CreateObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(req.GetObject())
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
	objects, err := s.funcs.Object.ListObjects(ctx, pbconv.ObjectFiltersFromProto(req.GetFilter())...)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListObjectsResponse{}
	for i := range objects {
		resp.Objects = append(resp.Objects, pbconv.ObjectToProto(&objects[i]))
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
	object, err := pbconv.ObjectFromProto(req.GetObject())
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
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)}, nil
}
func (s *Server) UpdateObjectManifest(ctx context.Context, req *sharedv1.UpdateObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	manifest, err := pbconv.ManifestFromProto(req.GetManifest())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Object.UpdateObjectManifest(ctx, req.GetObjectId(), manifest); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)}, nil
}
func (s *Server) WriteObjectFile(ctx context.Context, req *sharedv1.WriteObjectFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	if err := s.funcs.Object.WriteFile(ctx, req.GetObjectId(), req.GetFilename(), req.GetData()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	manifest, err := s.funcs.Object.GetObjectManifest(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)}, nil
}
func (s *Server) AppendObjectFile(ctx context.Context, req *sharedv1.WriteObjectFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	if err := s.funcs.Object.AppendFile(ctx, req.GetObjectId(), req.GetFilename(), req.GetData()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	manifest, err := s.funcs.Object.GetObjectManifest(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)}, nil
}
func (s *Server) ReadObjectFile(ctx context.Context, req *sharedv1.ReadObjectFileRequest) (*sharedv1.ObjectFileContent, error) {
	data, err := s.funcs.Object.ReadFile(ctx, req.GetObjectId(), req.GetFilename())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectFileContent{Data: data}, nil
}
func (s *Server) DeleteObjectFile(ctx context.Context, req *sharedv1.ReadObjectFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	if err := s.funcs.Object.DeleteFile(ctx, req.GetObjectId(), req.GetFilename()); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	manifest, err := s.funcs.Object.GetObjectManifest(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)}, nil
}
func (s *Server) ListObjectFiles(ctx context.Context, req *sharedv1.ListObjectFilesRequest) (*sharedv1.ListObjectFilesResponse, error) {
	files, err := s.funcs.Object.ListFiles(ctx, req.GetObjectId())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ListObjectFilesResponse{Filenames: files}, nil
}

func (s *Server) CreateTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := pbconv.TaskFromProto(req.GetTask())
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
	tasks, err := s.funcs.Task.ListTasks(ctx, pbconv.TaskFiltersFromProto(req.GetFilter())...)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListTasksResponse{}
	for i := range tasks {
		resp.Tasks = append(resp.Tasks, pbconv.TaskToProto(&tasks[i]))
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
	task, err := pbconv.TaskFromProto(req.GetTask())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	if err := s.funcs.Task.UpsertTask(ctx, task); err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}

func (s *Server) CreateObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := pbconv.ObservationFromProto(req.GetObservation())
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
	observations, err := s.funcs.Observation.ListObservations(ctx, pbconv.ObservationFiltersFromProto(req.GetFilter())...)
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	resp := &sharedv1.ListObservationsResponse{}
	for i := range observations {
		resp.Observations = append(resp.Observations, pbconv.ObservationToProto(&observations[i]))
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
	observation, err := pbconv.ObservationFromProto(req.GetObservation())
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
				if errors.Is(sub.Err(), changefeed.ErrSubscriberEvicted) {
					return status.Error(codes.ResourceExhausted, changefeed.ErrSubscriberEvicted.Error())
				}
				if err := sub.Err(); err != nil {
					return err
				}
				return nil
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
