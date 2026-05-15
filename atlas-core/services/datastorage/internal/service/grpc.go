package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/anomalyco/atlas-core/services/datastorage/internal/objectstorage"
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

const defaultObjectFileChunkSize = 64 * 1024

type receivedWriteFile struct {
	objectID     string
	filename     string
	expectedSize int64
}

type receivedAppendFile struct {
	receivedWriteFile
	currentExpectedSize int64
}

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
func (s *RPCServer) WriteObjectFile(stream datastoragev1.DataStorageService_WriteObjectFileServer) error {
	firstChunk, file, err := receiveFirstWriteChunk(stream)
	if err != nil {
		return err
	}
	manifest, err := s.svc.StreamWriteObjectFile(stream.Context(), file.objectID, file.filename, func(w io.Writer) error {
		return writeIncomingChunks(stream, w, file, firstChunk.GetData(), firstChunk.GetFinalChunk(), MAX_OBJECT_FILE_BYTES)
	})
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)})
}
func (s *RPCServer) AppendObjectFile(stream datastoragev1.DataStorageService_AppendObjectFileServer) error {
	firstChunk, file, err := receiveFirstAppendChunk(stream)
	if err != nil {
		return err
	}
	manifest, err := s.svc.StreamAppendObjectFile(
		stream.Context(),
		file.objectID,
		file.filename,
		file.currentExpectedSize,
		func(w io.Writer, currentSize int64) error {
			return appendIncomingChunks(stream, w, file, currentSize, firstChunk.GetData(), firstChunk.GetFinalChunk(), MAX_OBJECT_FILE_BYTES)
		},
	)
	if err != nil {
		var preconditionErr *objectstorage.AppendSizePreconditionError
		if errors.As(err, &preconditionErr) {
			return status.Error(codes.FailedPrecondition, preconditionErr.Error())
		}
		return rpcerrors.ToStatus(err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)})
}
func (s *RPCServer) ReadObjectFile(req *sharedv1.ReadFileRequest, stream datastoragev1.DataStorageService_ReadObjectFileServer) error {
	reader, totalSize, err := s.svc.OpenReadObjectFile(stream.Context(), req.GetObjectId(), req.GetFilename())
	if err != nil {
		return rpcerrors.ToStatus(err)
	}
	defer reader.Close()
	return sendObjectFileChunks(reader, totalSize, req.GetChunkSize(), stream.Send)
}
func (s *RPCServer) DeleteObjectFile(ctx context.Context, req *sharedv1.ReadFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	manifest, err := s.svc.DeleteObjectFile(ctx, req.GetObjectId(), req.GetFilename())
	if err != nil {
		return nil, rpcerrors.ToStatus(err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest)}, nil
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

func writeIncomingChunks(
	stream interface {
		Recv() (*sharedv1.WriteFileChunk, error)
	},
	writer io.Writer,
	file receivedWriteFile,
	firstData []byte,
	firstFinalChunk bool,
	maxBytes int64,
) error {
	totalBytes := int64(0)
	writeChunk := func(data []byte, finalChunk bool) error {
		totalBytes += int64(len(data))
		if totalBytes > maxBytes {
			return status.Error(codes.ResourceExhausted, fmt.Sprintf("object file exceeds maximum size of %d bytes", maxBytes))
		}
		if len(data) > 0 {
			if _, err := writer.Write(data); err != nil {
				return err
			}
		}
		if !finalChunk {
			return nil
		}
		if file.expectedSize != 0 && totalBytes != file.expectedSize {
			return status.Error(codes.InvalidArgument, fmt.Sprintf("expected_size mismatch: got %d bytes, expected %d", totalBytes, file.expectedSize))
		}
		if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
			if err != nil {
				return err
			}
			return status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		return nil
	}
	if err := writeChunk(firstData, firstFinalChunk); err != nil || firstFinalChunk {
		return err
	}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
		}
		if err != nil {
			return err
		}
		if err := validateWriteChunkMetadata(file, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
			return err
		}
		if err := writeChunk(chunk.GetData(), chunk.GetFinalChunk()); err != nil || chunk.GetFinalChunk() {
			return err
		}
	}
}

func appendIncomingChunks(
	stream interface {
		Recv() (*sharedv1.AppendFileChunk, error)
	},
	writer io.Writer,
	file receivedAppendFile,
	currentSize int64,
	firstData []byte,
	firstFinalChunk bool,
	maxBytes int64,
) error {
	totalBytes := int64(0)
	writeChunk := func(data []byte, finalChunk bool) error {
		totalBytes += int64(len(data))
		if currentSize+totalBytes > maxBytes {
			return status.Error(codes.ResourceExhausted, fmt.Sprintf("object file exceeds maximum size of %d bytes", maxBytes))
		}
		if len(data) > 0 {
			if _, err := writer.Write(data); err != nil {
				return err
			}
		}
		if !finalChunk {
			return nil
		}
		if file.expectedSize != 0 && currentSize+totalBytes != file.expectedSize {
			return status.Error(codes.InvalidArgument, fmt.Sprintf(
				"expected_size mismatch: got %d bytes after append, expected %d",
				currentSize+totalBytes, file.expectedSize))
		}
		if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
			if err != nil {
				return err
			}
			return status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		return nil
	}
	if err := writeChunk(firstData, firstFinalChunk); err != nil || firstFinalChunk {
		return err
	}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
		}
		if err != nil {
			return err
		}
		if err := validateWriteChunkMetadata(file.receivedWriteFile, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
			return err
		}
		if chunk.GetCurrentExpectedSize() != file.currentExpectedSize {
			return status.Error(codes.InvalidArgument, "current_expected_size must match across all chunks")
		}
		if err := writeChunk(chunk.GetData(), chunk.GetFinalChunk()); err != nil || chunk.GetFinalChunk() {
			return err
		}
	}
}

func sendObjectFileChunks(reader io.Reader, totalSize, chunkSize int64, send func(*sharedv1.FileChunk) error) error {
	if chunkSize <= 0 {
		chunkSize = defaultObjectFileChunkSize
	}
	if totalSize == 0 {
		return send(&sharedv1.FileChunk{FinalChunk: true, TotalSize: 0})
	}
	buffer := make([]byte, chunkSize)
	sentBytes := int64(0)
	for sentBytes < totalSize {
		n, err := reader.Read(buffer)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if n == 0 {
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}
		chunk := &sharedv1.FileChunk{
			Data: append([]byte(nil), buffer[:n]...),
		}
		sentBytes += int64(n)
		if sentBytes == int64(n) {
			chunk.TotalSize = totalSize
		}
		chunk.FinalChunk = sentBytes == totalSize
		if err := send(chunk); err != nil {
			return err
		}
		if chunk.FinalChunk {
			return nil
		}
	}
	return fmt.Errorf("object file stream truncated: sent %d of %d bytes", sentBytes, totalSize)
}
