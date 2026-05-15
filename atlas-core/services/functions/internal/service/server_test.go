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
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

const bufSize = 1024 * 1024

type fakeDataStorageServer struct {
	datastoragev1.UnimplementedDataStorageServiceServer
	mu       sync.Mutex
	entities map[string]*sharedv1.Entity
	objects  map[string]*sharedv1.Object
	files    map[string][]byte
}

func (s *fakeDataStorageServer) CreateEntity(_ context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity := req.GetEntity()
	clone := *entity
	clone.Version = 1
	s.entities[clone.GetEntityId()] = &clone
	return &sharedv1.EntityResponse{Entity: &clone}, nil
}

func (s *fakeDataStorageServer) GetEntity(_ context.Context, req *sharedv1.GetEntityRequest) (*sharedv1.EntityResponse, error) {
	entity, ok := s.entities[req.GetEntityId()]
	if !ok {
		return nil, model.ErrNotFound
	}
	clone := *entity
	return &sharedv1.EntityResponse{Entity: &clone}, nil
}

func (s *fakeDataStorageServer) GetObject(_ context.Context, req *sharedv1.GetObjectRequest) (*sharedv1.ObjectResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, ok := s.objects[req.GetObjectId()]
	if !ok {
		return nil, model.ErrNotFound
	}
	clone := *object
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
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{Manifest: s.manifestForObject(objectID)})
}

func (s *fakeDataStorageServer) AppendObjectFile(stream datastoragev1.DataStorageService_AppendObjectFileServer) error {
	chunk, err := stream.Recv()
	if err != nil {
		return err
	}
	if _, err := stream.Recv(); err != io.EOF {
		return fmt.Errorf("expected one append chunk, got %v", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[chunk.GetObjectId()]; !ok {
		return model.ErrNotFound
	}
	key := fmt.Sprintf("%s/%s", chunk.GetObjectId(), chunk.GetFilename())
	current := s.files[key]
	if int64(len(current)) != chunk.GetCurrentExpectedSize() {
		return model.NewCoreError("APPEND_SIZE_MISMATCH", "current_expected_size mismatch")
	}
	s.files[key] = append(append([]byte(nil), current...), chunk.GetData()...)
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{Manifest: s.manifestForObject(chunk.GetObjectId())})
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

func (s *fakeDataStorageServer) DeleteEntity(_ context.Context, req *sharedv1.DeleteEntityRequest) (*emptypb.Empty, error) {
	delete(s.entities, req.GetEntityId())
	return &emptypb.Empty{}, nil
}

func (s *fakeDataStorageServer) manifestForObject(objectID string) *sharedv1.ObjectManifest {
	manifest := &sharedv1.ObjectManifest{Version: "test", Files: map[string]*sharedv1.ObjectFileInfo{}}
	prefix := objectID + "/"
	for key, data := range s.files {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			manifest.Files[key[len(prefix):]] = &sharedv1.ObjectFileInfo{Size: int64(len(data))}
		}
	}
	return manifest
}

func startBufServer(t *testing.T, register func(*grpc.Server)) (*grpc.ClientConn, func()) {
	t.Helper()
	listener := bufconn.Listen(bufSize)
	server := grpc.NewServer()
	register(server)
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
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
		Object:      functionpkg.NewObjectFunctions(bundle.Object, datastorageclient.NopObjectStorageStore{}, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, logging.New("debug", "atlas-test", "test"), validator, hub),
	}

	funcConn, cleanupFunctions := startBufServer(t, func(server *grpc.Server) {
		RegisterGRPC(server, funcs, hub)
	})
	defer cleanupFunctions()

	client := functionsv1.NewAtlasFunctionsServiceClient(funcConn)
	streamClient := functionsv1.NewChangefeedServiceClient(funcConn)
	stream, err := streamClient.SubscribeMutations(context.Background(), &functionsv1.SubscribeMutationsRequest{})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	resp, err := client.CreateEntity(context.Background(), &sharedv1.EntityRequest{Entity: &sharedv1.Entity{
		EntityId: "asset-001",
		Type:     "asset",
		Json:     []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`),
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

func TestFunctionsServerStreamsObjectFiles(t *testing.T) {
	dsConn, cleanupDatastorage := startBufServer(t, func(server *grpc.Server) {
		datastoragev1.RegisterDataStorageServiceServer(server, &fakeDataStorageServer{
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
		Object:      functionpkg.NewObjectFunctions(bundle.Object, datastorageclient.NopObjectStorageStore{}, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, logging.New("debug", "atlas-test", "test"), validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, logging.New("debug", "atlas-test", "test"), validator, hub),
	}

	funcConn, cleanupFunctions := startBufServer(t, func(server *grpc.Server) {
		RegisterGRPC(server, funcs, hub)
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
}
func TestSubscribeMutationsReturnsResourceExhaustedWhenSubscriberEvicted(t *testing.T) {
	hub := changefeed.NewHub()
	server := NewServer(functionpkg.Functions{}, hub)
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
