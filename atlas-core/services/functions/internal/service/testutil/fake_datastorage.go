package testutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/objectstreaming"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FakeDataStorage struct {
	datastoragev1.UnimplementedDataStorageServiceServer
	Mu                sync.Mutex
	Entities          map[string]*sharedv1.Entity
	Objects           map[string]*sharedv1.Object
	Tasks             map[string]*sharedv1.Task
	Observations      map[string]*sharedv1.Observation
	Files             map[string][]byte
	WriteChunks       int
	AppendChunks      int
	ManifestSyncError string
}

func NewFakeDataStorage() *FakeDataStorage {
	return &FakeDataStorage{
		Entities:     map[string]*sharedv1.Entity{},
		Objects:      map[string]*sharedv1.Object{},
		Tasks:        map[string]*sharedv1.Task{},
		Observations: map[string]*sharedv1.Observation{},
		Files:        map[string][]byte{},
	}
}

func (s *FakeDataStorage) CreateEntity(_ context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity := req.GetEntity()
	clone := *entity
	clone.Version = 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Entities[clone.GetEntityId()] = &clone
	return &sharedv1.EntityResponse{Entity: &clone}, nil
}

func (s *FakeDataStorage) GetEntity(_ context.Context, req *sharedv1.GetEntityRequest) (*sharedv1.EntityResponse, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	entity, ok := s.Entities[req.GetEntityId()]
	if !ok {
		return nil, model.ErrNotFound
	}
	clone := *entity
	return &sharedv1.EntityResponse{Entity: &clone}, nil
}

func (s *FakeDataStorage) CreateObject(_ context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object := req.GetObject()
	clone := *object
	clone.Version = 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Objects[clone.GetObjectId()] = &clone
	return &sharedv1.ObjectResponse{Object: &clone}, nil
}

func (s *FakeDataStorage) UpsertObject(_ context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object := req.GetObject()
	clone := *object
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if existing, ok := s.Objects[clone.GetObjectId()]; ok {
		clone.Version = existing.GetVersion() + 1
		if clone.CreatedAt == nil {
			clone.CreatedAt = existing.GetCreatedAt()
		}
	} else {
		clone.Version = 1
		if clone.CreatedAt == nil {
			clone.CreatedAt = timestamppb.Now()
		}
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.Objects[clone.GetObjectId()] = &clone
	return &sharedv1.ObjectResponse{Object: &clone}, nil
}

func (s *FakeDataStorage) EnsureObjectCreated(_ context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object := req.GetObject()
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if existing, ok := s.Objects[object.GetObjectId()]; ok {
		clone := *existing
		return &sharedv1.ObjectResponse{Object: &clone}, nil
	}
	clone := *object
	clone.Version = 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.Objects[clone.GetObjectId()] = &clone
	return &sharedv1.ObjectResponse{Object: &clone}, nil
}

func (s *FakeDataStorage) UpdateObject(_ context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object := req.GetObject()
	s.Mu.Lock()
	defer s.Mu.Unlock()
	existing, ok := s.Objects[object.GetObjectId()]
	if !ok {
		return nil, rpcerrors.ToStatus(model.ErrNotFound)
	}
	if object.GetVersion() != existing.GetVersion() {
		return nil, rpcerrors.ToStatus(model.ErrVersionConflict)
	}
	clone := *object
	clone.Version = existing.GetVersion() + 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = existing.GetCreatedAt()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.Objects[clone.GetObjectId()] = &clone
	return &sharedv1.ObjectResponse{Object: &clone}, nil
}

func (s *FakeDataStorage) GetObject(_ context.Context, req *sharedv1.GetObjectRequest) (*sharedv1.ObjectResponse, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	object, ok := s.Objects[req.GetObjectId()]
	if !ok {
		return nil, model.ErrNotFound
	}
	clone := *object
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	return &sharedv1.ObjectResponse{Object: &clone}, nil
}

func (s *FakeDataStorage) GetObjectManifest(_ context.Context, req *sharedv1.GetObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if _, ok := s.Objects[req.GetObjectId()]; !ok {
		return nil, model.ErrNotFound
	}
	return &sharedv1.ObjectManifestResponse{Manifest: s.manifestForObject(req.GetObjectId())}, nil
}

func (s *FakeDataStorage) WriteObjectFile(stream datastoragev1.DataStorageService_WriteObjectFileServer) error {
	firstChunk, file, err := objectstreaming.ReceiveFirstWriteChunk(stream)
	if err != nil {
		return err
	}
	var data bytes.Buffer
	baseSink := objectstreaming.NewWriterSink(&data, file.ExpectedSize)
	countChunks := func(payload []byte, final bool, totalBytes int64) error {
		if len(payload) > 0 {
			s.Mu.Lock()
			s.WriteChunks++
			s.Mu.Unlock()
		}
		return baseSink(payload, final, totalBytes)
	}
	if err := objectstreaming.ProcessWriteChunks(
		stream.Recv,
		file,
		firstChunk.GetData(),
		firstChunk.GetFinalChunk(),
		objectstreaming.MaxChunkPayloadBytes,
		countChunks,
	); err != nil {
		return err
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if _, ok := s.Objects[file.ObjectID]; !ok {
		return model.ErrNotFound
	}
	s.Files[fmt.Sprintf("%s/%s", file.ObjectID, file.Filename)] = append([]byte(nil), data.Bytes()...)
	resp, err := s.manifestResponseLocked(file.ObjectID)
	if err != nil {
		return err
	}
	return stream.SendAndClose(resp)
}

func (s *FakeDataStorage) AppendObjectFile(stream datastoragev1.DataStorageService_AppendObjectFileServer) error {
	var (
		firstChunk *sharedv1.AppendFileChunk
		data       bytes.Buffer
	)
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		s.Mu.Lock()
		s.AppendChunks++
		s.Mu.Unlock()
		if firstChunk == nil {
			firstChunk = chunk
		}
		if _, err := data.Write(chunk.GetData()); err != nil {
			return err
		}
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if firstChunk == nil {
		return fmt.Errorf("expected at least one append chunk")
	}
	if _, ok := s.Objects[firstChunk.GetObjectId()]; !ok {
		return model.ErrNotFound
	}
	key := fmt.Sprintf("%s/%s", firstChunk.GetObjectId(), firstChunk.GetFilename())
	current := s.Files[key]
	if int64(len(current)) != firstChunk.GetCurrentExpectedSize() {
		return status.Error(codes.FailedPrecondition, "current_expected_size mismatch")
	}
	s.Files[key] = append(append([]byte(nil), current...), data.Bytes()...)
	resp, err := s.manifestResponseLocked(firstChunk.GetObjectId())
	if err != nil {
		return err
	}
	return stream.SendAndClose(resp)
}

func (s *FakeDataStorage) ReadObjectFile(req *sharedv1.ReadFileRequest, stream datastoragev1.DataStorageService_ReadObjectFileServer) error {
	s.Mu.Lock()
	if _, ok := s.Objects[req.GetObjectId()]; !ok {
		s.Mu.Unlock()
		return model.ErrNotFound
	}
	data := append([]byte(nil), s.Files[fmt.Sprintf("%s/%s", req.GetObjectId(), req.GetFilename())]...)
	s.Mu.Unlock()
	if len(data) == 0 {
		return stream.Send(&sharedv1.FileChunk{FinalChunk: true, TotalSize: 0})
	}
	for offset := 0; offset < len(data); offset += 3 {
		end := offset + 3
		if end > len(data) {
			end = len(data)
		}
		chunk := &sharedv1.FileChunk{Data: data[offset:end], FinalChunk: end == len(data)}
		if offset == 0 {
			chunk.TotalSize = int64(len(data))
		}
		if err := stream.Send(chunk); err != nil {
			return err
		}
	}
	return nil
}

func (s *FakeDataStorage) DeleteObjectFile(_ context.Context, req *sharedv1.ReadFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if _, ok := s.Objects[req.GetObjectId()]; !ok {
		return nil, model.ErrNotFound
	}
	delete(s.Files, fmt.Sprintf("%s/%s", req.GetObjectId(), req.GetFilename()))
	return s.manifestResponseLocked(req.GetObjectId())
}

func (s *FakeDataStorage) DeleteEntity(_ context.Context, req *sharedv1.DeleteEntityRequest) (*emptypb.Empty, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	delete(s.Entities, req.GetEntityId())
	return &emptypb.Empty{}, nil
}

func (s *FakeDataStorage) CreateTask(_ context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task := req.GetTask()
	clone := *task
	clone.Version = 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Tasks == nil {
		s.Tasks = map[string]*sharedv1.Task{}
	}
	s.Tasks[clone.GetTaskId()] = &clone
	return &sharedv1.TaskResponse{Task: &clone}, nil
}

func (s *FakeDataStorage) CreateObservation(_ context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation := req.GetObservation()
	clone := *observation
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Observations == nil {
		s.Observations = map[string]*sharedv1.Observation{}
	}
	if _, ok := s.Observations[clone.GetObservationId()]; ok {
		return nil, rpcerrors.ToStatus(model.ErrConflict)
	}
	clone.Version = 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.Observations[clone.GetObservationId()] = &clone
	return &sharedv1.ObservationResponse{Observation: &clone}, nil
}

func (s *FakeDataStorage) GetObservation(_ context.Context, req *sharedv1.GetObservationRequest) (*sharedv1.ObservationResponse, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	observation, ok := s.Observations[req.GetObservationId()]
	if !ok {
		return nil, rpcerrors.ToStatus(model.ErrNotFound)
	}
	clone := *observation
	return &sharedv1.ObservationResponse{Observation: &clone}, nil
}

func (s *FakeDataStorage) UpsertObservation(_ context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation := req.GetObservation()
	clone := *observation
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Observations == nil {
		s.Observations = map[string]*sharedv1.Observation{}
	}
	if existing, ok := s.Observations[clone.GetObservationId()]; ok {
		clone.Version = existing.GetVersion() + 1
		if clone.CreatedAt == nil {
			clone.CreatedAt = existing.GetCreatedAt()
		}
	} else {
		clone.Version = 1
		if clone.CreatedAt == nil {
			clone.CreatedAt = timestamppb.Now()
		}
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.Observations[clone.GetObservationId()] = &clone
	return &sharedv1.ObservationResponse{Observation: &clone}, nil
}

func (s *FakeDataStorage) UpdateObservation(_ context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation := req.GetObservation()
	s.Mu.Lock()
	defer s.Mu.Unlock()
	existing, ok := s.Observations[observation.GetObservationId()]
	if !ok {
		return nil, rpcerrors.ToStatus(model.ErrNotFound)
	}
	if observation.GetVersion() != existing.GetVersion() {
		return nil, rpcerrors.ToStatus(model.ErrVersionConflict)
	}
	clone := *observation
	clone.Version = existing.GetVersion() + 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = existing.GetCreatedAt()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.Observations[clone.GetObservationId()] = &clone
	return &sharedv1.ObservationResponse{Observation: &clone}, nil
}

func (s *FakeDataStorage) manifestForObject(objectID string) *sharedv1.ObjectManifest {
	manifest := &sharedv1.ObjectManifest{Version: "test", Files: map[string]*sharedv1.ObjectFileInfo{}}
	prefix := objectID + "/"
	for key, data := range s.Files {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			manifest.Files[key[len(prefix):]] = &sharedv1.ObjectFileInfo{Size: int64(len(data)), UpdatedAt: timestamppb.Now()}
		}
	}
	return manifest
}

func (s *FakeDataStorage) manifestResponse(objectID string) (*sharedv1.ObjectManifestResponse, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.manifestResponseLocked(objectID)
}

// manifestResponseLocked requires s.Mu held.
func (s *FakeDataStorage) manifestResponseLocked(objectID string) (*sharedv1.ObjectManifestResponse, error) {
	if s.ManifestSyncError != "" {
		return nil, status.Error(codes.Internal, s.ManifestSyncError)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: s.manifestForObject(objectID), ManifestCurrent: true}, nil
}
