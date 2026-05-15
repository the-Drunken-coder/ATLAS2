package service

import (
	"context"
	"net"
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

const bufSize = 1024 * 1024

type fakeDataStorageServer struct {
	datastoragev1.UnimplementedDataStorageServiceServer
	entities map[string]*sharedv1.Entity
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

func (s *fakeDataStorageServer) DeleteEntity(_ context.Context, req *sharedv1.DeleteEntityRequest) (*emptypb.Empty, error) {
	delete(s.entities, req.GetEntityId())
	return &emptypb.Empty{}, nil
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
		datastoragev1.RegisterDataStorageServiceServer(server, &fakeDataStorageServer{entities: map[string]*sharedv1.Entity{}})
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

	_, err = client.CreateEntity(context.Background(), &sharedv1.EntityRequest{Entity: &sharedv1.Entity{
		EntityId: "asset-001",
		Type:     "asset",
		Json:     []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`),
	}})
	if err != nil {
		t.Fatalf("create entity: %v", err)
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
	case err := <-errCh:
		t.Fatalf("recv event: %v", err)
	case <-recvCtx.Done():
		t.Fatal("timed out waiting for mutation event")
	}
}
