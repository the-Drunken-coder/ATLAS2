package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/functions/internal/changefeed"
	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	"github.com/anomalyco/atlas-core/services/functions/internal/service/testutil"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFunctionsServerStreamsMutationEvents(t *testing.T) {
	env := newFunctionsTestEnv(t, testutil.NewFakeDataStorage(), nil)
	client := env.Client
	streamClient := env.Changefeed
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
	fake := testutil.NewFakeDataStorage()
	fake.Objects["obj_001"] = &sharedv1.Object{
		ObjectId:  "obj_001",
		Type:      "log",
		OwnerType: "system",
		OwnerId:   "system",
		Json:      []byte(`{}`),
	}
	env := newFunctionsTestEnv(t, fake, nil)
	client := env.Client
	streamClient := env.Changefeed
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

func TestWriteObjectFileSucceedsWhenPublishFails(t *testing.T) {
	t.Run("bestEffortPublishObjectUpdated", func(t *testing.T) {
		log := logging.New("debug", "atlas-test", "test")
		bestEffortPublishObjectUpdated(context.Background(), log, "obj_001", nil)
		bestEffortPublishObjectUpdated(context.Background(), nil, "obj_001", errors.New("publish failed"))
	})

	fake := testutil.NewFakeDataStorage()
	fake.Objects["obj_001"] = &sharedv1.Object{
		ObjectId:  "obj_001",
		Type:      "log",
		OwnerType: "system",
		OwnerId:   "system",
		Json:      []byte(`{}`),
	}
	env := newFunctionsTestEnv(t, fake, func(handler *Server) {
		handler.testPublishObjectUpdated = func(context.Context, string) error {
			return errors.New("publish failed")
		}
	})
	client := env.Client
	writeStream, err := client.WriteObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open write stream: %v", err)
	}
	if err := writeStream.Send(&sharedv1.WriteFileChunk{
		ObjectId:     "obj_001",
		Filename:     "stream.txt",
		Data:         []byte("hello"),
		FinalChunk:   true,
		ExpectedSize: 5,
	}); err != nil {
		t.Fatalf("send write chunk: %v", err)
	}
	resp, err := writeStream.CloseAndRecv()
	if err != nil {
		t.Fatalf("close write stream: %v", err)
	}
	if resp.GetManifest() == nil {
		t.Fatal("expected manifest response after publish failure")
	}
}

func TestFunctionsServerDefaultsMissingTimestamps(t *testing.T) {
	now := timestamppb.Now()
	taskFake := testutil.NewFakeDataStorage()
	taskFake.Entities["asset-defaulted"] = &sharedv1.Entity{
		EntityId:  "asset-defaulted",
		Type:      "asset",
		Json:      []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`),
		CreatedAt: now,
		UpdatedAt: now,
	}
	taskFake.Objects["catalog-defaulted"] = &sharedv1.Object{
		ObjectId:  "catalog-defaulted",
		Type:      "command_catalog",
		OwnerType: "system",
		OwnerId:   "system",
		Json:      []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{}}]}`),
		CreatedAt: now,
		UpdatedAt: now,
	}

	tests := []struct {
		name string
		fake *testutil.FakeDataStorage
		run  func(t *testing.T, client functionsv1.AtlasFunctionsServiceClient)
	}{
		{
			name: "create entity",
			fake: testutil.NewFakeDataStorage(),
			run: func(t *testing.T, client functionsv1.AtlasFunctionsServiceClient) {
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
			},
		},
		{
			name: "upsert object",
			fake: testutil.NewFakeDataStorage(),
			run: func(t *testing.T, client functionsv1.AtlasFunctionsServiceClient) {
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
			},
		},
		{
			name: "create task",
			fake: taskFake,
			run: func(t *testing.T, client functionsv1.AtlasFunctionsServiceClient) {
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
			},
		},
		{
			name: "upsert observation",
			fake: testutil.NewFakeDataStorage(),
			run: func(t *testing.T, client functionsv1.AtlasFunctionsServiceClient) {
				resp, err := client.UpsertObservation(context.Background(), &sharedv1.ObservationRequest{Observation: &sharedv1.Observation{
					ObservationId: "obs-defaulted",
					SourceAssetId: "asset-defaulted",
					StartedAt:     timestamppb.New(time.Now().UTC()),
					Json:          []byte(`{"identity":{"kind":"asset"}}`),
				}})
				if err != nil {
					t.Fatalf("upsert observation without timestamps: %v", err)
				}
				if resp.GetObservation().GetCreatedAt() == nil || resp.GetObservation().GetUpdatedAt() == nil {
					t.Fatalf("expected server defaults for timestamps, got %+v", resp.GetObservation())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newFunctionsTestEnv(t, tt.fake, nil)
			tt.run(t, env.Client)
		})
	}
}

func TestFunctionsServerStreamsObjectFiles(t *testing.T) {
	fake := testutil.NewFakeDataStorage()
	fake.Objects["obj_001"] = &sharedv1.Object{
		ObjectId:  "obj_001",
		Type:      "log",
		OwnerType: "system",
		OwnerId:   "system",
		Json:      []byte(`{}`),
	}
	env := newFunctionsTestEnv(t, fake, nil)
	client := env.Client

	writeStream, err := client.WriteObjectFile(context.Background())
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
	if env.Fake.WriteChunks < 3 {
		t.Fatalf("expected forwarded multi-chunk write, got %d chunks", env.Fake.WriteChunks)
	}

	readStream, err := client.ReadObjectFile(context.Background(), &sharedv1.ReadFileRequest{ObjectId: "obj_001", Filename: "data.txt", ChunkSize: 2})
	if err != nil {
		t.Fatalf("open read stream through functions proxy: %v", err)
	}
	if _, err := readStream.Recv(); err != nil {
		t.Fatalf("recv read chunk through functions proxy: %v", err)
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
	if env.Fake.AppendChunks < 3 {
		t.Fatalf("expected forwarded multi-chunk append, got %d chunks", env.Fake.AppendChunks)
	}

	env.Fake.Mu.Lock()
	env.Fake.ManifestSyncError = "manifest sync failed"
	env.Fake.Mu.Unlock()

	writeStream, err = client.WriteObjectFile(context.Background())
	if err != nil {
		t.Fatalf("open manifest-failure write stream: %v", err)
	}
	if err := writeStream.Send(&sharedv1.WriteFileChunk{
		ObjectId:     "obj_001",
		Filename:     "partial.txt",
		Data:         []byte("payload"),
		FinalChunk:   true,
		ExpectedSize: 7,
	}); err != nil {
		t.Fatalf("send manifest-failure write chunk: %v", err)
	}
	if _, err := writeStream.CloseAndRecv(); err == nil {
		t.Fatal("expected manifest rebuild failure on write close")
	}

	if _, err := client.DeleteObjectFile(context.Background(), &sharedv1.ReadFileRequest{ObjectId: "obj_001", Filename: "partial.txt"}); err == nil {
		t.Fatal("expected manifest rebuild failure on delete")
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
