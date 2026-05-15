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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const MAX_OBJECT_FILE_BYTES = 4*1024*1024 - 4096 // 4 MiB − 4 KiB

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
func (s *Server) WriteObjectFile(stream functionsv1.AtlasFunctionsService_WriteObjectFileServer) error {
	file, err := receiveWriteObjectFileChunks(stream)
	if err != nil {
		return err
	}
	if err := s.funcs.Object.WriteFile(stream.Context(), file.objectID, file.filename, file.data); err != nil {
		return rpcerrors.ToStatus(err)
	}
	manifest, err := s.funcs.Object.GetObjectManifest(stream.Context(), file.objectID)
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)})
}
func (s *Server) AppendObjectFile(stream functionsv1.AtlasFunctionsService_AppendObjectFileServer) error {
	file, err := receiveAppendObjectFileChunks(stream, s.currentObjectFileSize)
	if err != nil {
		return err
	}
	if err := s.funcs.Object.AppendFile(stream.Context(), file.objectID, file.filename, file.data); err != nil {
		return rpcerrors.ToStatus(err)
	}
	manifest, err := s.funcs.Object.GetObjectManifest(stream.Context(), file.objectID)
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)})
}
func (s *Server) ReadObjectFile(req *sharedv1.ReadFileRequest, stream functionsv1.AtlasFunctionsService_ReadObjectFileServer) error {
	data, err := s.funcs.Object.ReadFile(stream.Context(), req.GetObjectId(), req.GetFilename())
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	return sendObjectFileChunks(data, req.GetChunkSize(), stream.Send)
}
func (s *Server) DeleteObjectFile(ctx context.Context, req *sharedv1.ReadFileRequest) (*sharedv1.ObjectManifestResponse, error) {
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
	for event := range s.hub.Subscribe(stream.Context()) {
		if err := stream.Send(event); err != nil {
			return err
		}
	}
	return nil
}

type receivedWriteFile struct {
	objectID     string
	filename     string
	data         []byte
	expectedSize int64
}

type receivedAppendFile struct {
	receivedWriteFile
	currentExpectedSize int64
	currentSize         int64
}

func (s *Server) currentObjectFileSize(ctx context.Context, objectID, filename string) (int64, error) {
	manifest, err := s.funcs.Object.GetObjectManifest(ctx, objectID)
	if err != nil {
		return 0, err
	}
	info, ok := manifest.Files[filename]
	if !ok {
		return 0, nil
	}
	return info.Size, nil
}

func receiveWriteObjectFileChunks(stream interface {
	Context() context.Context
	Recv() (*sharedv1.WriteFileChunk, error)
}) (receivedWriteFile, error) {
	var (
		file        receivedWriteFile
		receivedAny bool
		sawFinal    bool
		totalBytes  int64
	)
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			if !receivedAny {
				return receivedWriteFile{}, status.Error(codes.InvalidArgument, "at least one chunk is required")
			}
			if !sawFinal {
				return receivedWriteFile{}, status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
			}
			if file.expectedSize != 0 && totalBytes != file.expectedSize {
				return receivedWriteFile{}, status.Error(codes.InvalidArgument, fmt.Sprintf(
					"expected_size mismatch: got %d bytes, expected %d",
					totalBytes, file.expectedSize))
			}
			return file, nil
		}
		if err != nil {
			return receivedWriteFile{}, err
		}
		if sawFinal {
			return receivedWriteFile{}, status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		if err := applyWriteChunk(&file, chunk, receivedAny, &totalBytes); err != nil {
			return receivedWriteFile{}, err
		}
		receivedAny = true
		sawFinal = chunk.GetFinalChunk()
	}
}

func receiveAppendObjectFileChunks(
	stream interface {
		Context() context.Context
		Recv() (*sharedv1.AppendFileChunk, error)
	},
	currentSize func(context.Context, string, string) (int64, error),
) (receivedAppendFile, error) {
	var (
		file        receivedAppendFile
		receivedAny bool
		sawFinal    bool
		totalBytes  int64
	)
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			if !receivedAny {
				return receivedAppendFile{}, status.Error(codes.InvalidArgument, "at least one chunk is required")
			}
			if !sawFinal {
				return receivedAppendFile{}, status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
			}
			totalSize := file.currentSize + totalBytes
			if file.expectedSize != 0 && totalSize != file.expectedSize {
				return receivedAppendFile{}, status.Error(codes.InvalidArgument, fmt.Sprintf(
					"expected_size mismatch: got %d bytes after append, expected %d",
					totalSize, file.expectedSize))
			}
			return file, nil
		}
		if err != nil {
			return receivedAppendFile{}, err
		}
		if sawFinal {
			return receivedAppendFile{}, status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		if !receivedAny {
			file.objectID = chunk.GetObjectId()
			file.filename = chunk.GetFilename()
			file.expectedSize = chunk.GetExpectedSize()
			file.currentExpectedSize = chunk.GetCurrentExpectedSize()
			if file.objectID == "" {
				return receivedAppendFile{}, status.Error(codes.InvalidArgument, "object_id is required")
			}
			if file.filename == "" {
				return receivedAppendFile{}, status.Error(codes.InvalidArgument, "filename is required")
			}
			file.currentSize, err = currentSize(stream.Context(), file.objectID, file.filename)
			if err != nil {
				return receivedAppendFile{}, rpcerrors.ToStatus(err)
			}
			if file.currentSize != file.currentExpectedSize {
				return receivedAppendFile{}, status.Error(codes.FailedPrecondition, fmt.Sprintf(
					"current_expected_size mismatch: actual %d, expected %d",
					file.currentSize, file.currentExpectedSize))
			}
		} else if err := validateWriteChunkMetadata(
			file.receivedWriteFile,
			chunk.GetObjectId(),
			chunk.GetFilename(),
			chunk.GetExpectedSize(),
		); err != nil {
			return receivedAppendFile{}, err
		}
		if file.currentSize+totalBytes+int64(len(chunk.GetData())) > MAX_OBJECT_FILE_BYTES {
			return receivedAppendFile{}, status.Error(codes.ResourceExhausted, fmt.Sprintf(
				"object file exceeds maximum size of %d bytes",
				MAX_OBJECT_FILE_BYTES))
		}
		file.data = append(file.data, chunk.GetData()...)
		totalBytes += int64(len(chunk.GetData()))
		receivedAny = true
		sawFinal = chunk.GetFinalChunk()
	}
}

func applyWriteChunk(file *receivedWriteFile, chunk *sharedv1.WriteFileChunk, receivedAny bool, totalBytes *int64) error {
	if !receivedAny {
		file.objectID = chunk.GetObjectId()
		file.filename = chunk.GetFilename()
		file.expectedSize = chunk.GetExpectedSize()
		if file.objectID == "" {
			return status.Error(codes.InvalidArgument, "object_id is required")
		}
		if file.filename == "" {
			return status.Error(codes.InvalidArgument, "filename is required")
		}
	} else if err := validateWriteChunkMetadata(*file, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
		return err
	}
	if *totalBytes+int64(len(chunk.GetData())) > MAX_OBJECT_FILE_BYTES {
		return status.Error(codes.ResourceExhausted, fmt.Sprintf(
			"object file exceeds maximum size of %d bytes",
			MAX_OBJECT_FILE_BYTES))
	}
	file.data = append(file.data, chunk.GetData()...)
	*totalBytes += int64(len(chunk.GetData()))
	return nil
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

func sendObjectFileChunks(data []byte, chunkSize int64, send func(*sharedv1.FileChunk) error) error {
	if chunkSize <= 0 {
		chunkSize = 64 * 1024
	}
	totalSize := int64(len(data))
	if totalSize == 0 {
		return send(&sharedv1.FileChunk{FinalChunk: true, TotalSize: 0})
	}
	for offset := 0; offset < len(data); offset += int(chunkSize) {
		end := offset + int(chunkSize)
		if end > len(data) {
			end = len(data)
		}
		chunk := &sharedv1.FileChunk{
			Data:       data[offset:end],
			FinalChunk: end == len(data),
		}
		if offset == 0 {
			chunk.TotalSize = totalSize
		}
		if err := send(chunk); err != nil {
			return err
		}
	}
	return nil
}
