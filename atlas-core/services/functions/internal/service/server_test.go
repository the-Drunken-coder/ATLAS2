package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/functions/internal/changefeed"
	"github.com/anomalyco/atlas-core/services/functions/internal/datastorageclient"
	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const bufSize = 1024 * 1024

type fakeDataStorageServer struct {
	datastoragev1.UnimplementedDataStorageServiceServer
	mu                sync.Mutex
	entities          map[string]*sharedv1.Entity
	objects           map[string]*sharedv1.Object
	tasks             map[string]*sharedv1.Task
	observations      map[string]*sharedv1.Observation
	files             map[string][]byte
	writeChunks       int
	appendChunks      int
	manifestSyncError string
}

func (s *fakeDataStorageServer) CreateEntity(_ context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity := req.GetEntity()
	clone := *entity
	clone.Version = 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entities[clone.GetEntityId()] = &clone
	return &sharedv1.EntityResponse{Entity: &clone}, nil
}

func (s *fakeDataStorageServer) GetEntity(_ context.Context, req *sharedv1.GetEntityRequest) (*sharedv1.EntityResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entity, ok := s.entities[req.GetEntityId()]
	if !ok {
		return nil, model.ErrNotFound
	}
	clone := *entity
	return &sharedv1.EntityResponse{Entity: &clone}, nil
}

func (s *fakeDataStorageServer) CreateObject(_ context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object := req.GetObject()
	clone := *object
	clone.Version = 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[clone.GetObjectId()] = &clone
	return &sharedv1.ObjectResponse{Object: &clone}, nil
}

func (s *fakeDataStorageServer) UpsertObject(_ context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object := req.GetObject()
	clone := *object
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.objects[clone.GetObjectId()]; ok {
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
	s.objects[clone.GetObjectId()] = &clone
	return &sharedv1.ObjectResponse{Object: &clone}, nil
}

func (s *fakeDataStorageServer) GetObject(_ context.Context, req *sharedv1.GetObjectRequest) (*sharedv1.ObjectResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, ok := s.objects[req.GetObjectId()]
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

func (s *fakeDataStorageServer) GetObjectManifest(_ context.Context, req *sharedv1.GetObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[req.GetObjectId()]; !ok {
		return nil, model.ErrNotFound
	}
	return &sharedv1.ObjectManifestResponse{Manifest: s.manifestForObject(req.GetObjectId())}, nil
}

func (s *fakeDataStorageServer) WriteObjectFile(stream datastoragev1.DataStorageService_WriteObjectFileServer) error {
	var objectID, filename string
	var data bytes.Buffer
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.writeChunks++
		s.mu.Unlock()
		objectID = chunk.GetObjectId()
		filename = chunk.GetFilename()
		if _, err := data.Write(chunk.GetData()); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[objectID]; !ok {
		return model.ErrNotFound
	}
	s.files[fmt.Sprintf("%s/%s", objectID, filename)] = append([]byte(nil), data.Bytes()...)
	return stream.SendAndClose(s.manifestResponse(objectID))
}

func (s *fakeDataStorageServer) AppendObjectFile(stream datastoragev1.DataStorageService_AppendObjectFileServer) error {
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
		s.mu.Lock()
		s.appendChunks++
		s.mu.Unlock()
		if firstChunk == nil {
			firstChunk = chunk
		}
		if _, err := data.Write(chunk.GetData()); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if firstChunk == nil {
		return fmt.Errorf("expected at least one append chunk")
	}
	if _, ok := s.objects[firstChunk.GetObjectId()]; !ok {
		return model.ErrNotFound
	}
	key := fmt.Sprintf("%s/%s", firstChunk.GetObjectId(), firstChunk.GetFilename())
	current := s.files[key]
	if int64(len(current)) != firstChunk.GetCurrentExpectedSize() {
		return status.Error(codes.FailedPrecondition, "current_expected_size mismatch")
	}
	s.files[key] = append(append([]byte(nil), current...), data.Bytes()...)
	return stream.SendAndClose(s.manifestResponse(firstChunk.GetObjectId()))
}

func (s *fakeDataStorageServer) ReadObjectFile(req *sharedv1.ReadFileRequest, stream datastoragev1.DataStorageService_ReadObjectFileServer) error {
	s.mu.Lock()
	if _, ok := s.objects[req.GetObjectId()]; !ok {
		s.mu.Unlock()
		return model.ErrNotFound
	}
	data := append([]byte(nil), s.files[fmt.Sprintf("%s/%s", req.GetObjectId(), req.GetFilename())]...)
	s.mu.Unlock()
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

func (s *fakeDataStorageServer) DeleteObjectFile(_ context.Context, req *sharedv1.ReadFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[req.GetObjectId()]; !ok {
		return nil, model.ErrNotFound
	}
	delete(s.files, fmt.Sprintf("%s/%s", req.GetObjectId(), req.GetFilename()))
	return s.manifestResponse(req.GetObjectId()), nil
}

func (s *fakeDataStorageServer) DeleteEntity(_ context.Context, req *sharedv1.DeleteEntityRequest) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entities, req.GetEntityId())
	return &emptypb.Empty{}, nil
}

func (s *fakeDataStorageServer) CreateTask(_ context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task := req.GetTask()
	clone := *task
	clone.Version = 1
	if clone.CreatedAt == nil {
		clone.CreatedAt = timestamppb.Now()
	}
	if clone.UpdatedAt == nil {
		clone.UpdatedAt = timestamppb.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tasks == nil {
		s.tasks = map[string]*sharedv1.Task{}
	}
	s.tasks[clone.GetTaskId()] = &clone
	return &sharedv1.TaskResponse{Task: &clone}, nil
}

func (s *fakeDataStorageServer) UpsertObservation(_ context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation := req.GetObservation()
	clone := *observation
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.observations == nil {
		s.observations = map[string]*sharedv1.Observation{}
	}
	if existing, ok := s.observations[clone.GetObservationId()]; ok {
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
	s.observations[clone.GetObservationId()] = &clone
	return &sharedv1.ObservationResponse{Observation: &clone}, nil
}

func (s *fakeDataStorageServer) manifestForObject(objectID string) *sharedv1.ObjectManifest {
	manifest := &sharedv1.ObjectManifest{Version: "test", Files: map[string]*sharedv1.ObjectFileInfo{}}
	prefix := objectID + "/"
	for key, data := range s.files {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			manifest.Files[key[len(prefix):]] = &sharedv1.ObjectFileInfo{Size: int64(len(data)), UpdatedAt: timestamppb.Now()}
		}
	}
	return manifest
}

func (s *fakeDataStorageServer) manifestResponse(objectID string) *sharedv1.ObjectManifestResponse {
	resp := &sharedv1.ObjectManifestResponse{Manifest: s.manifestForObject(objectID), ManifestCurrent: true}
	if s.manifestSyncError != "" {
		resp.ManifestCurrent = false
		resp.ManifestSyncError = s.manifestSyncError
	}
	return resp
}

func startBufServer(t *testing.T, register func(*grpc.Server)) (*grpc.ClientConn, func()) {
	t.Helper()
	listener := bufconn.Listen(bufSize)
	server := grpc.NewServer()
	register(server)
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	cleanup := func() {
		conn.Close()
		server.Stop()
		listener.Close()
	}
	return conn, cleanup
}

func TestFunctionsServerStreamsMutationEvents(t *testing.T) {
	dsConn, cleanupDatastorage := startBufServer(t, func(server *grpc.Server) {
		datastoragev1.RegisterDataStorageServiceServer(server, &fakeDataStorageServer{
			entities: map[string]*sharedv1.Entity{},
			objects:  map[string]*sharedv1.Object{},
			files:    map[string][]byte{},
		})
	})
	defer cleanupDatastorage()

	validator, err := protocolvalidation.New()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	bundle := datastorageclient.New(datastoragev1.NewDataStorageServiceClient(dsConn))
	hub := changefeed.NewHub()
	funcs := functionpkg.Functions{
		Entity:      functionpkg.NewEntityFunctions(bundle.Entity, logging.New("debug", "atlas-test", "test"), validator, hub),
		Object:      functionpkg.NewObjectFunctions(bundle.Object, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, logging.New("debug", "atlas-test", "test"), validator, hub),
	}

	funcConn, cleanupFunctions := startBufServer(t, func(server *grpc.Server) {
		RegisterGRPC(server, funcs, hub, nil)
	})
	defer cleanupFunctions()

	client := functionsv1.NewAtlasFunctionsServiceClient(funcConn)
	streamClient := functionsv1.NewChangefeedServiceClient(funcConn)
	stream, err := streamClient.SubscribeMutations(context.Background(), &functionsv1.SubscribeMutationsRequest{})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	resp, err := client.CreateEntity(context.Background(), &sharedv1.EntityRequest{Entity: &sharedv1.Entity{
		EntityId:  "asset-001",
		Type:      "asset",
		Json:      []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`),
		CreatedAt: timestamppb.Now(),
		UpdatedAt: timestamppb.Now(),
	}})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	if resp.GetEntity().GetVersion() != 1 {
		t.Fatalf("expected create response version 1, got %d", resp.GetEntity().GetVersion())
	}

	recvCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	eventCh := make(chan *sharedv1.MutationEvent, 1)
	errCh := make(chan error, 1)
	go func() {
		event, recvErr := stream.Recv()
		if recvErr != nil {
			errCh <- recvErr
			return
		}
		eventCh <- event
	}()

	select {
	case event := <-eventCh:
		if event.GetResource() != "entity" || event.GetOperation() != "created" || event.GetResourceId() != "asset-001" {
			t.Fatalf("unexpected event: %+v", event)
		}
		if event.GetResourceVersion() != 1 {
			t.Fatalf("expected event resource version 1, got %d", event.GetResourceVersion())
		}
		if event.GetEntity().GetVersion() != 1 {
			t.Fatalf("expected event snapshot version 1, got %d", event.GetEntity().GetVersion())
		}
	case err := <-errCh:
		t.Fatalf("recv event: %v", err)
	case <-recvCtx.Done():
		t.Fatal("timed out waiting for mutation event")
	}
}

func TestFunctionsServerStreamingFileMutationsPublishChangefeed(t *testing.T) {
	dsServer := &fakeDataStorageServer{
		entities: map[string]*sharedv1.Entity{},
		objects: map[string]*sharedv1.Object{
			"obj_001": {
				ObjectId:  "obj_001",
				Type:      "log",
				OwnerType: "system",
				OwnerId:   "system",
				Json:      []byte(`{}`),
			},
		},
		files: map[string][]byte{},
	}
	dsConn, cleanupDatastorage := startBufServer(t, func(server *grpc.Server) {
		datastoragev1.RegisterDataStorageServiceServer(server, dsServer)
	})
	defer cleanupDatastorage()

	validator, err := protocolvalidation.New()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	bundle := datastorageclient.New(datastoragev1.NewDataStorageServiceClient(dsConn))
	hub := changefeed.NewHub()
	funcs := functionpkg.Functions{
		Entity:      functionpkg.NewEntityFunctions(bundle.Entity, logging.New("debug", "atlas-test", "test"), validator, hub),
		Object:      functionpkg.NewObjectFunctions(bundle.Object, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, logging.New("debug", "atlas-test", "test"), validator, hub),
	}

	funcConn, cleanupFunctions := startBufServer(t, func(server *grpc.Server) {
		RegisterGRPC(server, funcs, hub, nil)
	})
	defer cleanupFunctions()

	client := functionsv1.NewAtlasFunctionsServiceClient(funcConn)
	streamClient := functionsv1.NewChangefeedServiceClient(funcConn)
	sub, err := streamClient.SubscribeMutations(context.Background(), &functionsv1.SubscribeMutationsRequest{})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	recvObjectUpdated := func(t *testing.T) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		errCh := make(chan error, 1)
		evCh := make(chan *sharedv1.MutationEvent, 1)
		go func() {
			ev, recvErr := sub.Recv()
			if recvErr != nil {
				errCh <- recvErr
				return
			}
			evCh <- ev
		}()
		select {
		case err := <-errCh:
			t.Fatalf("recv: %v", err)
		case ev := <-evCh:
			if ev.GetResource() != "object" || ev.GetOperation() != "updated" || ev.GetResourceId() != "obj_001" {
				t.Fatalf("unexpected event: resource=%q op=%q id=%q", ev.GetResource(), ev.GetOperation(), ev.GetResourceId())
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for object updated mutation")
		}
	}

	writeStream, err := client.WriteObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open write stream: %v", err)
	}
	for i, chunk := range [][]byte{[]byte("abc"), []byte("def"), []byte("ghi")} {
		if err := writeStream.Send(&sharedv1.WriteFileChunk{
			ObjectId:     "obj_001",
			Filename:     "stream.txt",
			Data:         chunk,
			FinalChunk:   i == 2,
			ExpectedSize: 9,
		}); err != nil {
			t.Fatalf("send write chunk %d: %v", i, err)
		}
	}
	if _, err := writeStream.CloseAndRecv(); err != nil {
		t.Fatalf("close write stream: %v", err)
	}
	recvObjectUpdated(t)

	appendStream, err := client.AppendObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open append stream: %v", err)
	}
	for i, chunk := range [][]byte{[]byte("jkl"), []byte("mn")} {
		if err := appendStream.Send(&sharedv1.AppendFileChunk{
			ObjectId:            "obj_001",
			Filename:            "stream.txt",
			Data:                chunk,
			FinalChunk:          i == 1,
			CurrentExpectedSize: 9,
			ExpectedSize:        14,
		}); err != nil {
			t.Fatalf("send append chunk %d: %v", i, err)
		}
	}
	if _, err := appendStream.CloseAndRecv(); err != nil {
		t.Fatalf("close append stream: %v", err)
	}
	recvObjectUpdated(t)

	appendStream, err = client.AppendObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open stale append stream: %v", err)
	}
	if err := appendStream.Send(&sharedv1.AppendFileChunk{
		ObjectId:            "obj_001",
		Filename:            "stream.txt",
		Data:                []byte("abc"),
		FinalChunk:          true,
		CurrentExpectedSize: 1,
		ExpectedSize:        4,
	}); err != nil {
		t.Fatalf("send stale append chunk: %v", err)
	}
	if _, err := appendStream.CloseAndRecv(); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition on stale append, got %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		_, recvErr := sub.Recv()
		errCh <- recvErr
	}()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected Recv error while waiting for no event: %v", err)
		}
		t.Fatal("expected no changefeed event after failed stale append, got event")
	case <-ctx.Done():
		// no event within timeout — expected
	}
}

func TestFunctionsServerCreateEntityDefaultsMissingTimestamps(t *testing.T) {
	dsConn, cleanupDatastorage := startBufServer(t, func(server *grpc.Server) {
		datastoragev1.RegisterDataStorageServiceServer(server, &fakeDataStorageServer{
			entities: map[string]*sharedv1.Entity{},
			objects:  map[string]*sharedv1.Object{},
			files:    map[string][]byte{},
		})
	})
	defer cleanupDatastorage()

	validator, err := protocolvalidation.New()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	bundle := datastorageclient.New(datastoragev1.NewDataStorageServiceClient(dsConn))
	hub := changefeed.NewHub()
	funcs := functionpkg.Functions{
		Entity:      functionpkg.NewEntityFunctions(bundle.Entity, logging.New("debug", "atlas-test", "test"), validator, hub),
		Object:      functionpkg.NewObjectFunctions(bundle.Object, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, logging.New("debug", "atlas-test", "test"), validator, hub),
	}

	funcConn, cleanupFunctions := startBufServer(t, func(server *grpc.Server) {
		RegisterGRPC(server, funcs, hub, nil)
	})
	defer cleanupFunctions()

	client := functionsv1.NewAtlasFunctionsServiceClient(funcConn)
	resp, err := client.CreateEntity(context.Background(), &sharedv1.EntityRequest{Entity: &sharedv1.Entity{
		EntityId: "asset-defaulted",
		Type:     "asset",
		Json:     []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`),
	}})
	if err != nil {
		t.Fatalf("create entity without timestamps: %v", err)
	}
	if resp.GetEntity().GetCreatedAt() == nil || resp.GetEntity().GetUpdatedAt() == nil {
		t.Fatalf("expected server defaults for timestamps, got %+v", resp.GetEntity())
	}
}

func TestFunctionsServerUpsertObjectDefaultsMissingTimestamps(t *testing.T) {
	dsConn, cleanupDatastorage := startBufServer(t, func(server *grpc.Server) {
		datastoragev1.RegisterDataStorageServiceServer(server, &fakeDataStorageServer{
			entities: map[string]*sharedv1.Entity{},
			objects:  map[string]*sharedv1.Object{},
			files:    map[string][]byte{},
		})
	})
	defer cleanupDatastorage()

	validator, err := protocolvalidation.New()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	bundle := datastorageclient.New(datastoragev1.NewDataStorageServiceClient(dsConn))
	hub := changefeed.NewHub()
	funcs := functionpkg.Functions{
		Entity:      functionpkg.NewEntityFunctions(bundle.Entity, logging.New("debug", "atlas-test", "test"), validator, hub),
		Object:      functionpkg.NewObjectFunctions(bundle.Object, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, logging.New("debug", "atlas-test", "test"), validator, hub),
	}

	funcConn, cleanupFunctions := startBufServer(t, func(server *grpc.Server) {
		RegisterGRPC(server, funcs, hub, nil)
	})
	defer cleanupFunctions()

	client := functionsv1.NewAtlasFunctionsServiceClient(funcConn)
	resp, err := client.UpsertObject(context.Background(), &sharedv1.ObjectRequest{Object: &sharedv1.Object{
		ObjectId:  "obj_defaulted",
		Type:      "log",
		OwnerType: "system",
		OwnerId:   "system",
		Json:      []byte(`{}`),
	}})
	if err != nil {
		t.Fatalf("upsert object without timestamps: %v", err)
	}
	if resp.GetObject().GetCreatedAt() == nil || resp.GetObject().GetUpdatedAt() == nil {
		t.Fatalf("expected server defaults for timestamps, got %+v", resp.GetObject())
	}
}

func TestFunctionsServerCreateTaskDefaultsMissingTimestamps(t *testing.T) {
	dsConn, cleanupDatastorage := startBufServer(t, func(server *grpc.Server) {
		datastoragev1.RegisterDataStorageServiceServer(server, &fakeDataStorageServer{
			entities: map[string]*sharedv1.Entity{
				"asset-defaulted": {
					EntityId:  "asset-defaulted",
					Type:      "asset",
					Json:      []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`),
					CreatedAt: timestamppb.Now(),
					UpdatedAt: timestamppb.Now(),
				},
			},
			objects: map[string]*sharedv1.Object{
				"catalog-defaulted": {
					ObjectId:  "catalog-defaulted",
					Type:      "command_catalog",
					OwnerType: "system",
					OwnerId:   "system",
					Json:      []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{}}]}`),
					CreatedAt: timestamppb.Now(),
					UpdatedAt: timestamppb.Now(),
				},
			},
			files: map[string][]byte{},
		})
	})
	defer cleanupDatastorage()

	validator, err := protocolvalidation.New()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	bundle := datastorageclient.New(datastoragev1.NewDataStorageServiceClient(dsConn))
	hub := changefeed.NewHub()
	funcs := functionpkg.Functions{
		Entity:      functionpkg.NewEntityFunctions(bundle.Entity, logging.New("debug", "atlas-test", "test"), validator, hub),
		Object:      functionpkg.NewObjectFunctions(bundle.Object, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, logging.New("debug", "atlas-test", "test"), validator, hub),
	}

	funcConn, cleanupFunctions := startBufServer(t, func(server *grpc.Server) {
		RegisterGRPC(server, funcs, hub, nil)
	})
	defer cleanupFunctions()

	client := functionsv1.NewAtlasFunctionsServiceClient(funcConn)
	resp, err := client.CreateTask(context.Background(), &sharedv1.TaskRequest{Task: &sharedv1.Task{
		TaskId:                 "task-defaulted",
		Status:                 "pending",
		AssetId:                "asset-defaulted",
		CommandCatalogObjectId: "catalog-defaulted",
		Json:                   []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{}}}`),
	}})
	if err != nil {
		t.Fatalf("create task without timestamps: %v", err)
	}
	if resp.GetTask().GetCreatedAt() == nil || resp.GetTask().GetUpdatedAt() == nil {
		t.Fatalf("expected server defaults for timestamps, got %+v", resp.GetTask())
	}
}

func TestFunctionsServerUpsertObservationDefaultsMissingTimestamps(t *testing.T) {
	dsConn, cleanupDatastorage := startBufServer(t, func(server *grpc.Server) {
		datastoragev1.RegisterDataStorageServiceServer(server, &fakeDataStorageServer{
			entities: map[string]*sharedv1.Entity{},
			objects:  map[string]*sharedv1.Object{},
			files:    map[string][]byte{},
		})
	})
	defer cleanupDatastorage()

	validator, err := protocolvalidation.New()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	bundle := datastorageclient.New(datastoragev1.NewDataStorageServiceClient(dsConn))
	hub := changefeed.NewHub()
	funcs := functionpkg.Functions{
		Entity:      functionpkg.NewEntityFunctions(bundle.Entity, logging.New("debug", "atlas-test", "test"), validator, hub),
		Object:      functionpkg.NewObjectFunctions(bundle.Object, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, logging.New("debug", "atlas-test", "test"), validator, hub),
	}

	funcConn, cleanupFunctions := startBufServer(t, func(server *grpc.Server) {
		RegisterGRPC(server, funcs, hub, nil)
	})
	defer cleanupFunctions()

	client := functionsv1.NewAtlasFunctionsServiceClient(funcConn)
	resp, err := client.UpsertObservation(context.Background(), &sharedv1.ObservationRequest{Observation: &sharedv1.Observation{
		ObservationId: "obs-defaulted",
		SourceAssetId: "asset-defaulted",
		Json:          []byte(`{"state":"active"}`),
	}})
	if err != nil {
		t.Fatalf("upsert observation without timestamps: %v", err)
	}
	if resp.GetObservation().GetCreatedAt() == nil || resp.GetObservation().GetUpdatedAt() == nil {
		t.Fatalf("expected server defaults for timestamps, got %+v", resp.GetObservation())
	}
}

func TestFunctionsServerStreamsObjectFiles(t *testing.T) {
	dsServer := &fakeDataStorageServer{
		entities: map[string]*sharedv1.Entity{},
		objects: map[string]*sharedv1.Object{
			"obj_001": {
				ObjectId:  "obj_001",
				Type:      "log",
				OwnerType: "system",
				OwnerId:   "system",
				Json:      []byte(`{}`),
			},
		},
		files: map[string][]byte{},
	}
	dsConn, cleanupDatastorage := startBufServer(t, func(server *grpc.Server) {
		datastoragev1.RegisterDataStorageServiceServer(server, dsServer)
	})
	defer cleanupDatastorage()

	validator, err := protocolvalidation.New()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	bundle := datastorageclient.New(datastoragev1.NewDataStorageServiceClient(dsConn))
	hub := changefeed.NewHub()
	funcs := functionpkg.Functions{
		Entity:      functionpkg.NewEntityFunctions(bundle.Entity, logging.New("debug", "atlas-test", "test"), validator, hub),
		Object:      functionpkg.NewObjectFunctions(bundle.Object, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, logging.New("debug", "atlas-test", "test"), validator, hub),
	}

	funcConn, cleanupFunctions := startBufServer(t, func(server *grpc.Server) {
		RegisterGRPC(server, funcs, hub, nil)
	})
	defer cleanupFunctions()

	client := functionsv1.NewAtlasFunctionsServiceClient(funcConn)

	writeStream, err := client.WriteObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open write stream: %v", err)
	}
	if err := writeStream.Send(&sharedv1.WriteFileChunk{
		ObjectId:   "obj_001",
		Filename:   "data.txt",
		FinalChunk: true,
	}); err != nil {
		t.Fatalf("send empty write chunk: %v", err)
	}
	if _, err := writeStream.CloseAndRecv(); err != nil {
		t.Fatalf("close empty write stream: %v", err)
	}

	readStream, err := client.ReadObjectFile(context.Background(), &sharedv1.ReadFileRequest{ObjectId: "obj_001", Filename: "data.txt", ChunkSize: 2})
	if err != nil {
		t.Fatalf("open read stream: %v", err)
	}
	firstChunk, err := readStream.Recv()
	if err != nil {
		t.Fatalf("recv empty read chunk: %v", err)
	}
	if !firstChunk.GetFinalChunk() || firstChunk.GetTotalSize() != 0 {
		t.Fatalf("unexpected empty file chunk: %+v", firstChunk)
	}

	writeStream, err = client.WriteObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open multi write stream: %v", err)
	}
	for i, chunk := range [][]byte{[]byte("abc"), []byte("def"), []byte("ghi")} {
		if err := writeStream.Send(&sharedv1.WriteFileChunk{
			ObjectId:     "obj_001",
			Filename:     "data.txt",
			Data:         chunk,
			FinalChunk:   i == 2,
			ExpectedSize: 9,
		}); err != nil {
			t.Fatalf("send multi write chunk %d: %v", i, err)
		}
	}
	writeResp, err := writeStream.CloseAndRecv()
	if err != nil {
		t.Fatalf("close multi write stream: %v", err)
	}
	if !writeResp.GetManifestCurrent() || writeResp.GetManifestSyncError() != "" {
		t.Fatalf("expected current manifest after write, got %+v", writeResp)
	}
	if dsServer.writeChunks < 4 {
		t.Fatalf("expected forwarded multi-chunk write, got %d chunks", dsServer.writeChunks)
	}

	readStream, err = client.ReadObjectFile(context.Background(), &sharedv1.ReadFileRequest{ObjectId: "obj_001", Filename: "data.txt", ChunkSize: 2})
	if err != nil {
		t.Fatalf("open multi read stream: %v", err)
	}
	var readBack bytes.Buffer
	for {
		chunk, err := readStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("recv multi read chunk: %v", err)
		}
		if _, err := readBack.Write(chunk.GetData()); err != nil {
			t.Fatalf("buffer multi read chunk: %v", err)
		}
		if chunk.GetFinalChunk() {
			break
		}
	}
	if got := readBack.String(); got != "abcdefghi" {
		t.Fatalf("expected abcdefghi, got %q", got)
	}

	appendStream, err := client.AppendObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open append stream: %v", err)
	}
	if err := appendStream.Send(&sharedv1.AppendFileChunk{
		ObjectId:            "obj_001",
		Filename:            "data.txt",
		Data:                []byte("abc"),
		FinalChunk:          true,
		CurrentExpectedSize: 1,
		ExpectedSize:        4,
	}); err != nil {
		t.Fatalf("send append chunk: %v", err)
	}
	if _, err := appendStream.CloseAndRecv(); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}

	appendStream, err = client.AppendObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open append success stream: %v", err)
	}
	for i, chunk := range [][]byte{[]byte("jkl"), []byte("mn")} {
		if err := appendStream.Send(&sharedv1.AppendFileChunk{
			ObjectId:            "obj_001",
			Filename:            "data.txt",
			Data:                chunk,
			FinalChunk:          i == 1,
			CurrentExpectedSize: 9,
			ExpectedSize:        14,
		}); err != nil {
			t.Fatalf("send append success chunk %d: %v", i, err)
		}
	}
	appendResp, err := appendStream.CloseAndRecv()
	if err != nil {
		t.Fatalf("close append success stream: %v", err)
	}
	if !appendResp.GetManifestCurrent() || appendResp.GetManifestSyncError() != "" {
		t.Fatalf("expected current manifest after append, got %+v", appendResp)
	}
	if dsServer.appendChunks < 3 {
		t.Fatalf("expected forwarded multi-chunk append, got %d chunks", dsServer.appendChunks)
	}

	dsServer.mu.Lock()
	dsServer.manifestSyncError = "manifest sync failed"
	dsServer.mu.Unlock()

	writeStream, err = client.WriteObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open partial write stream: %v", err)
	}
	if err := writeStream.Send(&sharedv1.WriteFileChunk{
		ObjectId:     "obj_001",
		Filename:     "partial.txt",
		Data:         []byte("payload"),
		FinalChunk:   true,
		ExpectedSize: 7,
	}); err != nil {
		t.Fatalf("send partial write chunk: %v", err)
	}
	partialWriteResp, err := writeStream.CloseAndRecv()
	if err != nil {
		t.Fatalf("close partial write stream: %v", err)
	}
	if partialWriteResp.GetManifestCurrent() {
		t.Fatalf("expected stale manifest after partial write, got %+v", partialWriteResp)
	}
	if partialWriteResp.GetManifestSyncError() != "manifest sync failed" {
		t.Fatalf("expected stable manifest sync error, got %+v", partialWriteResp)
	}

	deleteResp, err := client.DeleteObjectFile(context.Background(), &sharedv1.ReadFileRequest{ObjectId: "obj_001", Filename: "partial.txt"})
	if err != nil {
		t.Fatalf("delete partial file: %v", err)
	}
	if deleteResp.GetManifestCurrent() {
		t.Fatalf("expected stale manifest after partial delete, got %+v", deleteResp)
	}
	if deleteResp.GetManifestSyncError() != "manifest sync failed" {
		t.Fatalf("expected stable manifest sync error on delete, got %+v", deleteResp)
	}
}
func TestSubscribeMutationsReturnsResourceExhaustedWhenSubscriberEvicted(t *testing.T) {
	hub := changefeed.NewHub()
	server := NewServer(functionpkg.Functions{}, hub, nil)
	stream := newBlockingMutationStream(1)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.SubscribeMutations(&functionsv1.SubscribeMutationsRequest{}, stream)
	}()

	stream.publishUntilSent(t, hub, &sharedv1.MutationEvent{EventId: "initial", Resource: "entity", Operation: "updated"})

	hub.Publish(context.Background(), &sharedv1.MutationEvent{EventId: "blocker", Resource: "entity", Operation: "updated"})
	stream.waitUntilBlocked(t)

	for i := 0; i < 64; i++ {
		hub.Publish(context.Background(), &sharedv1.MutationEvent{EventId: "overflow", Resource: "entity", Operation: "updated"})
	}

	stream.unblock()

	select {
	case err := <-errCh:
		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}
		if st.Code() != codes.ResourceExhausted {
			t.Fatalf("expected ResourceExhausted, got %v", st.Code())
		}
		if st.Message() != changefeed.ErrSubscriberEvicted.Error() {
			t.Fatalf("expected exact error text %q, got %q", changefeed.ErrSubscriberEvicted.Error(), st.Message())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for SubscribeMutations to return")
	}
}

func TestNewServerInitializesHubWhenNil(t *testing.T) {
	server := NewServer(functionpkg.Functions{}, nil, nil)
	if server.hub == nil {
		t.Fatal("expected NewServer to provide a non-nil hub")
	}
}

type blockingMutationStream struct {
	ctx        context.Context
	unblockCh  chan struct{}
	blockedCh  chan struct{}
	sendCh     chan int
	blockAfter int

	mu        sync.Mutex
	sendCount int
	blocked   bool
}

func newBlockingMutationStream(blockAfter int) *blockingMutationStream {
	ctx := context.Background()
	return &blockingMutationStream{
		ctx:        ctx,
		unblockCh:  make(chan struct{}),
		blockedCh:  make(chan struct{}),
		sendCh:     make(chan int, 128),
		blockAfter: blockAfter,
	}
}

func (s *blockingMutationStream) Send(*sharedv1.MutationEvent) error {
	s.mu.Lock()
	s.sendCount++
	count := s.sendCount
	shouldBlock := !s.blocked && count > s.blockAfter
	if shouldBlock {
		s.blocked = true
	}
	s.mu.Unlock()

	select {
	case s.sendCh <- count:
	default:
	}

	if shouldBlock {
		close(s.blockedCh)
		<-s.unblockCh
	}
	return nil
}

func (s *blockingMutationStream) Context() context.Context     { return s.ctx }
func (s *blockingMutationStream) SetHeader(metadata.MD) error  { return nil }
func (s *blockingMutationStream) SendHeader(metadata.MD) error { return nil }
func (s *blockingMutationStream) SetTrailer(metadata.MD)       {}
func (s *blockingMutationStream) SendMsg(any) error            { return nil }
func (s *blockingMutationStream) RecvMsg(any) error            { return nil }

func (s *blockingMutationStream) publishUntilSent(t *testing.T, hub *changefeed.Hub, event *sharedv1.MutationEvent) {
	t.Helper()

	deadline := time.After(time.Second)
	for {
		hub.Publish(context.Background(), event)
		select {
		case got := <-s.sendCh:
			if got >= 1 {
				return
			}
		case <-time.After(10 * time.Millisecond):
		case <-deadline:
			t.Fatal("timed out waiting for initial streamed event")
		}
	}
}

func (s *blockingMutationStream) waitUntilBlocked(t *testing.T) {
	t.Helper()

	select {
	case <-s.blockedCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream to block")
	}
}

func (s *blockingMutationStream) unblock() {
	close(s.unblockCh)
}
