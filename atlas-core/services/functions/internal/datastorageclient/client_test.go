package datastorageclient

import (
	"context"
	"net"
	"testing"
	"time"

	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const testBufSize = 1024 * 1024

type versioningDataStorageServer struct {
	datastoragev1.UnimplementedDataStorageServiceServer
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

func cloneEntityWithVersion(entity *sharedv1.Entity, version int32) *sharedv1.Entity {
	clone := *entity
	clone.Version = version
	return &clone
}

func cloneTaskWithVersion(task *sharedv1.Task, version int32) *sharedv1.Task {
	clone := *task
	clone.Version = version
	return &clone
}

func cloneObservationWithVersion(observation *sharedv1.Observation, version int32) *sharedv1.Observation {
	clone := *observation
	clone.Version = version
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

	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
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
