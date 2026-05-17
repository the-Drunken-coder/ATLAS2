package datastorageclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const testBufSize = 1024 * 1024

type versioningDataStorageServer struct {
	datastoragev1.UnimplementedDataStorageServiceServer
}

type fileStreamingDataStorageServer struct {
	datastoragev1.UnimplementedDataStorageServiceServer
	mu                sync.Mutex
	files             map[string][]byte
	lastAppendRequest *sharedv1.AppendFileChunk
	writeChunkCount   int
	appendChunkCount  int
	manifestSyncError string
}

func (s versioningDataStorageServer) CreateEntity(_ context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity := cloneEntityWithVersion(req.GetEntity(), 11)
	return &sharedv1.EntityResponse{Entity: entity}, nil
}

func (s versioningDataStorageServer) UpdateEntity(_ context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity := cloneEntityWithVersion(req.GetEntity(), 12)
	return &sharedv1.EntityResponse{Entity: entity}, nil
}

func (s versioningDataStorageServer) UpsertEntity(_ context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity := cloneEntityWithVersion(req.GetEntity(), 13)
	return &sharedv1.EntityResponse{Entity: entity}, nil
}

func (s versioningDataStorageServer) CreateTask(_ context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task := cloneTaskWithVersion(req.GetTask(), 21)
	return &sharedv1.TaskResponse{Task: task}, nil
}

func (s versioningDataStorageServer) UpdateTask(_ context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task := cloneTaskWithVersion(req.GetTask(), 22)
	return &sharedv1.TaskResponse{Task: task}, nil
}

func (s versioningDataStorageServer) UpsertTask(_ context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task := cloneTaskWithVersion(req.GetTask(), 23)
	return &sharedv1.TaskResponse{Task: task}, nil
}

func (s versioningDataStorageServer) CreateObservation(_ context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation := cloneObservationWithVersion(req.GetObservation(), 31)
	return &sharedv1.ObservationResponse{Observation: observation}, nil
}

func (s versioningDataStorageServer) UpdateObservation(_ context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation := cloneObservationWithVersion(req.GetObservation(), 32)
	return &sharedv1.ObservationResponse{Observation: observation}, nil
}

func (s versioningDataStorageServer) UpsertObservation(_ context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation := cloneObservationWithVersion(req.GetObservation(), 33)
	return &sharedv1.ObservationResponse{Observation: observation}, nil
}

func (s *fileStreamingDataStorageServer) WriteObjectFile(stream datastoragev1.DataStorageService_WriteObjectFileServer) error {
	var objectID, filename string
	var data bytes.Buffer
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		objectID = chunk.GetObjectId()
		filename = chunk.GetFilename()
		s.writeChunkCount++
		if _, err := data.Write(chunk.GetData()); err != nil {
			return err
		}
	}
	key := fmt.Sprintf("%s/%s", objectID, filename)
	s.mu.Lock()
	s.files[key] = append([]byte(nil), data.Bytes()...)
	s.mu.Unlock()
	return stream.SendAndClose(s.manifestResponse(objectID))
}

func (s *fileStreamingDataStorageServer) AppendObjectFile(stream datastoragev1.DataStorageService_AppendObjectFileServer) error {
	var (
		firstChunk *sharedv1.AppendFileChunk
		data       bytes.Buffer
	)
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		s.appendChunkCount++
		if firstChunk == nil {
			firstChunk = chunk
		}
		if _, err := data.Write(chunk.GetData()); err != nil {
			return err
		}
		if chunk.GetFinalChunk() {
			break
		}
	}
	if firstChunk == nil {
		return fmt.Errorf("expected at least one append chunk")
	}
	key := fmt.Sprintf("%s/%s", firstChunk.GetObjectId(), firstChunk.GetFilename())
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.files[key]
	if int64(len(current)) != firstChunk.GetCurrentExpectedSize() {
		return fmt.Errorf("current_expected_size mismatch: got %d want %d", firstChunk.GetCurrentExpectedSize(), len(current))
	}
	s.lastAppendRequest = firstChunk
	s.files[key] = append(append([]byte(nil), current...), data.Bytes()...)
	return stream.SendAndClose(s.manifestResponse(firstChunk.GetObjectId()))
}

func (s *fileStreamingDataStorageServer) ReadObjectFile(req *sharedv1.ReadFileRequest, stream datastoragev1.DataStorageService_ReadObjectFileServer) error {
	key := fmt.Sprintf("%s/%s", req.GetObjectId(), req.GetFilename())
	s.mu.Lock()
	data := append([]byte(nil), s.files[key]...)
	s.mu.Unlock()
	if len(data) == 0 {
		return stream.Send(&sharedv1.FileChunk{FinalChunk: true, TotalSize: 0})
	}
	for offset := 0; offset < len(data); offset += 2 {
		end := offset + 2
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

func (s *fileStreamingDataStorageServer) DeleteObjectFile(_ context.Context, req *sharedv1.ReadFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	key := fmt.Sprintf("%s/%s", req.GetObjectId(), req.GetFilename())
	s.mu.Lock()
	delete(s.files, key)
	s.mu.Unlock()
	return s.manifestResponse(req.GetObjectId()), nil
}

func (s *fileStreamingDataStorageServer) GetObjectManifest(_ context.Context, req *sharedv1.GetObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	return s.manifestResponse(req.GetObjectId()), nil
}

func (s *fileStreamingDataStorageServer) manifestForObject(objectID string) *sharedv1.ObjectManifest {
	manifest := &sharedv1.ObjectManifest{Version: "test", Files: map[string]*sharedv1.ObjectFileInfo{}}
	prefix := objectID + "/"
	for key, data := range s.files {
		if filename, ok := strings.CutPrefix(key, prefix); ok {
			manifest.Files[filename] = &sharedv1.ObjectFileInfo{Size: int64(len(data)), UpdatedAt: timestamppb.Now()}
		}
	}
	return manifest
}

func (s *fileStreamingDataStorageServer) manifestResponse(objectID string) *sharedv1.ObjectManifestResponse {
	resp := &sharedv1.ObjectManifestResponse{Manifest: s.manifestForObject(objectID), ManifestCurrent: true}
	if s.manifestSyncError != "" {
		resp.ManifestCurrent = false
		resp.ManifestSyncError = s.manifestSyncError
	}
	return resp
}

func cloneEntityWithVersion(entity *sharedv1.Entity, version int32) *sharedv1.Entity {
	clone := *entity
	clone.Version = version
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	return &clone
}

func cloneTaskWithVersion(task *sharedv1.Task, version int32) *sharedv1.Task {
	clone := *task
	clone.Version = version
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	return &clone
}

func cloneObservationWithVersion(observation *sharedv1.Observation, version int32) *sharedv1.Observation {
	clone := *observation
	clone.Version = version
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	return &clone
}

func startVersioningDataStorageClient(t *testing.T) datastoragev1.DataStorageServiceClient {
	t.Helper()

	listener := bufconn.Listen(testBufSize)
	server := grpc.NewServer()
	datastoragev1.RegisterDataStorageServiceServer(server, versioningDataStorageServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		listener.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return datastoragev1.NewDataStorageServiceClient(conn)
}

func startStreamingDataStorageClient(t *testing.T, serverImpl datastoragev1.DataStorageServiceServer) datastoragev1.DataStorageServiceClient {
	t.Helper()

	listener := bufconn.Listen(testBufSize)
	server := grpc.NewServer()
	datastoragev1.RegisterDataStorageServiceServer(server, serverImpl)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		listener.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return datastoragev1.NewDataStorageServiceClient(conn)
}

func TestClientsCopyReturnedVersions(t *testing.T) {
	bundle := New(startVersioningDataStorageClient(t))
	now := time.Now().UTC()

	entity := &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := bundle.Entity.CreateEntity(context.Background(), entity); err != nil {
		t.Fatalf("create entity: %v", err)
	}
	if entity.Version != 11 {
		t.Fatalf("expected create entity version 11, got %d", entity.Version)
	}
	if err := bundle.Entity.UpdateEntity(context.Background(), entity); err != nil {
		t.Fatalf("update entity: %v", err)
	}
	if entity.Version != 12 {
		t.Fatalf("expected update entity version 12, got %d", entity.Version)
	}
	if err := bundle.Entity.UpsertEntity(context.Background(), entity); err != nil {
		t.Fatalf("upsert entity: %v", err)
	}
	if entity.Version != 13 {
		t.Fatalf("expected upsert entity version 13, got %d", entity.Version)
	}

	task := &model.Task{TaskID: "task_001", Status: model.TaskStatusPending, AssetID: "asset_001", CommandCatalogObjectID: "catalog_001", JSON: []byte(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := bundle.Task.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.Version != 21 {
		t.Fatalf("expected create task version 21, got %d", task.Version)
	}
	if err := bundle.Task.UpdateTask(context.Background(), task); err != nil {
		t.Fatalf("update task: %v", err)
	}
	if task.Version != 22 {
		t.Fatalf("expected update task version 22, got %d", task.Version)
	}
	if err := bundle.Task.UpsertTask(context.Background(), task); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	if task.Version != 23 {
		t.Fatalf("expected upsert task version 23, got %d", task.Version)
	}

	observation := &model.Observation{ObservationID: "obs_001", SourceAssetID: "asset_001", JSON: []byte(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := bundle.Observation.CreateObservation(context.Background(), observation); err != nil {
		t.Fatalf("create observation: %v", err)
	}
	if observation.Version != 31 {
		t.Fatalf("expected create observation version 31, got %d", observation.Version)
	}
	if err := bundle.Observation.UpdateObservation(context.Background(), observation); err != nil {
		t.Fatalf("update observation: %v", err)
	}
	if observation.Version != 32 {
		t.Fatalf("expected update observation version 32, got %d", observation.Version)
	}
	if err := bundle.Observation.UpsertObservation(context.Background(), observation); err != nil {
		t.Fatalf("upsert observation: %v", err)
	}
	if observation.Version != 33 {
		t.Fatalf("expected upsert observation version 33, got %d", observation.Version)
	}
}

func TestObjectGatewayClientStreamsFileRPCs(t *testing.T) {
	server := &fileStreamingDataStorageServer{files: map[string][]byte{}}
	bundle := New(startStreamingDataStorageClient(t, server))

	if result, err := bundle.Object.WriteFile(context.Background(), "obj_001", "data.txt", []byte("")); err != nil {
		t.Fatalf("write empty file: %v", err)
	} else if !result.ManifestCurrent || result.ManifestSyncError != "" {
		t.Fatalf("expected current manifest after empty write, got %+v", result)
	}
	data, err := bundle.Object.ReadFile(context.Background(), "obj_001", "data.txt")
	if err != nil {
		t.Fatalf("read empty file: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty file, got %q", string(data))
	}

	if result, err := bundle.Object.WriteFile(context.Background(), "obj_001", "data.txt", []byte("hello")); err != nil {
		t.Fatalf("write file: %v", err)
	} else if !result.ManifestCurrent || result.ManifestSyncError != "" {
		t.Fatalf("expected current manifest after write, got %+v", result)
	}
	if result, err := bundle.Object.AppendFile(context.Background(), "obj_001", "data.txt", []byte(" world")); err != nil {
		t.Fatalf("append file: %v", err)
	} else if !result.ManifestCurrent || result.ManifestSyncError != "" {
		t.Fatalf("expected current manifest after append, got %+v", result)
	}
	data, err = bundle.Object.ReadFile(context.Background(), "obj_001", "data.txt")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := string(data); got != "hello world" {
		t.Fatalf("expected combined file, got %q", got)
	}
	if server.lastAppendRequest == nil {
		t.Fatal("expected append request to be captured")
	}
	if server.lastAppendRequest.GetCurrentExpectedSize() != 5 {
		t.Fatalf("expected current_expected_size 5, got %d", server.lastAppendRequest.GetCurrentExpectedSize())
	}
	if server.lastAppendRequest.GetExpectedSize() != 11 {
		t.Fatalf("expected expected_size 11, got %d", server.lastAppendRequest.GetExpectedSize())
	}

	largePayload := bytes.Repeat([]byte("a"), defaultWriteObjectChunkSize*2+10)
	prevWriteChunks := server.writeChunkCount
	if _, err := bundle.Object.WriteFile(context.Background(), "obj_001", "large.txt", largePayload); err != nil {
		t.Fatalf("write large file: %v", err)
	}
	if got := server.writeChunkCount - prevWriteChunks; got < 3 {
		t.Fatalf("expected multi-chunk write, got %d new chunks", got)
	}

	prevAppendChunks := server.appendChunkCount
	if _, err := bundle.Object.AppendFile(context.Background(), "obj_001", "large.txt", largePayload[:defaultWriteObjectChunkSize+5]); err != nil {
		t.Fatalf("append large file: %v", err)
	}
	if got := server.appendChunkCount - prevAppendChunks; got < 2 {
		t.Fatalf("expected multi-chunk append, got %d new chunks", got)
	}

	server.manifestSyncError = "manifest sync failed"
	staleWrite, err := bundle.Object.WriteFile(context.Background(), "obj_001", "stale.txt", []byte("payload"))
	if err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	if staleWrite.ManifestCurrent {
		t.Fatalf("expected stale manifest after write, got %+v", staleWrite)
	}
	if staleWrite.ManifestSyncError != "manifest sync failed" {
		t.Fatalf("expected stable manifest sync error after write, got %+v", staleWrite)
	}

	staleAppend, err := bundle.Object.AppendFile(context.Background(), "obj_001", "stale.txt", []byte(" more"))
	if err != nil {
		t.Fatalf("append stale file: %v", err)
	}
	if staleAppend.ManifestCurrent {
		t.Fatalf("expected stale manifest after append, got %+v", staleAppend)
	}
	if staleAppend.ManifestSyncError != "manifest sync failed" {
		t.Fatalf("expected stable manifest sync error after append, got %+v", staleAppend)
	}

	staleDelete, err := bundle.Object.DeleteFile(context.Background(), "obj_001", "stale.txt")
	if err != nil {
		t.Fatalf("delete stale file: %v", err)
	}
	if staleDelete.ManifestCurrent {
		t.Fatalf("expected stale manifest after delete, got %+v", staleDelete)
	}
	if staleDelete.ManifestSyncError != "manifest sync failed" {
		t.Fatalf("expected stable manifest sync error after delete, got %+v", staleDelete)
	}
}
