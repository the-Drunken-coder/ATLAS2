package service

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/datastorage/internal/objectstorage"
	"github.com/anomalyco/atlas-core/services/datastorage/internal/postgres"
	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const integrationBufSize = 1024 * 1024

func startDatastorageTestClient(t *testing.T) (datastoragev1.DataStorageServiceClient, *Service) {
	t.Helper()

	pool := testPool(t)
	t.Cleanup(pool.Close)

	log := logging.New("debug", "atlas-test", "test")
	storage := objectstorage.NewStore(t.TempDir(), log)
	if err := storage.InitRoot(); err != nil {
		t.Fatalf("init object storage: %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	svc := &Service{
		Logger:        log,
		objectStore:   postgres.NewObjectStore(pool, log),
		objectStorage: storage,
	}

	now := time.Now().UTC()
	if err := svc.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create object: %v", err)
	}

	listener := bufconn.Listen(integrationBufSize)
	server := grpc.NewServer()
	RegisterGRPC(server, svc)
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
	t.Cleanup(func() { _ = conn.Close() })

	return datastoragev1.NewDataStorageServiceClient(conn), svc
}

func TestDataStorageStreamsObjectFiles(t *testing.T) {
	client, svc := startDatastorageTestClient(t)

	t.Run("multi chunk write", func(t *testing.T) {
		stream, err := client.WriteObjectFile(context.Background())
		if err != nil {
			t.Fatalf("open write stream: %v", err)
		}
		chunks := [][]byte{[]byte("ab"), []byte("cd"), []byte("ef")}
		for i, chunk := range chunks {
			if err := stream.Send(&sharedv1.WriteFileChunk{
				ObjectId:     "obj_001",
				Filename:     "data.txt",
				Data:         chunk,
				FinalChunk:   i == len(chunks)-1,
				ExpectedSize: 6,
			}); err != nil {
				t.Fatalf("send chunk %d: %v", i, err)
			}
		}
		if _, err := stream.CloseAndRecv(); err != nil {
			t.Fatalf("close write stream: %v", err)
		}
		data, err := svc.ReadObjectFile(context.Background(), "obj_001", "data.txt")
		if err != nil {
			t.Fatalf("read object file: %v", err)
		}
		if got := string(data); got != "abcdef" {
			t.Fatalf("expected abcdef, got %q", got)
		}
	})

	t.Run("multi chunk read", func(t *testing.T) {
		payload := bytes.Repeat([]byte("z"), 2500)
		if _, err := svc.WriteObjectFile(context.Background(), "obj_001", "read.txt", payload); err != nil {
			t.Fatalf("seed read file: %v", err)
		}
		stream, err := client.ReadObjectFile(context.Background(), &sharedv1.ReadFileRequest{ObjectId: "obj_001", Filename: "read.txt", ChunkSize: 1024})
		if err != nil {
			t.Fatalf("open read stream: %v", err)
		}
		var out bytes.Buffer
		chunks := 0
		for {
			chunk, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("recv read chunk: %v", err)
			}
			chunks++
			if chunks == 1 && chunk.GetTotalSize() != int64(len(payload)) {
				t.Fatalf("expected total size %d, got %d", len(payload), chunk.GetTotalSize())
			}
			if _, err := out.Write(chunk.GetData()); err != nil {
				t.Fatalf("buffer read chunk: %v", err)
			}
			if chunk.GetFinalChunk() {
				break
			}
		}
		if chunks != 3 {
			t.Fatalf("expected 3 chunks, got %d", chunks)
		}
		if !bytes.Equal(out.Bytes(), payload) {
			t.Fatal("read payload mismatch")
		}
	})

	t.Run("multi chunk append success", func(t *testing.T) {
		if _, err := svc.WriteObjectFile(context.Background(), "obj_001", "append.txt", []byte("base")); err != nil {
			t.Fatalf("seed append file: %v", err)
		}
		stream, err := client.AppendObjectFile(context.Background())
		if err != nil {
			t.Fatalf("open append stream: %v", err)
		}
		appendChunks := [][]byte{[]byte("-one"), []byte("-two")}
		for i, chunk := range appendChunks {
			if err := stream.Send(&sharedv1.AppendFileChunk{
				ObjectId:            "obj_001",
				Filename:            "append.txt",
				Data:                chunk,
				FinalChunk:          i == len(appendChunks)-1,
				CurrentExpectedSize: 4,
				ExpectedSize:        12,
			}); err != nil {
				t.Fatalf("send append chunk %d: %v", i, err)
			}
		}
		if _, err := stream.CloseAndRecv(); err != nil {
			t.Fatalf("close append stream: %v", err)
		}
		data, err := svc.ReadObjectFile(context.Background(), "obj_001", "append.txt")
		if err != nil {
			t.Fatalf("read appended file: %v", err)
		}
		if got := string(data); got != "base-one-two" {
			t.Fatalf("expected base-one-two, got %q", got)
		}
	})

	t.Run("stale append wrong size fails", func(t *testing.T) {
		if _, err := svc.WriteObjectFile(context.Background(), "obj_001", "stale.txt", []byte("hello")); err != nil {
			t.Fatalf("seed stale file: %v", err)
		}
		stream, err := client.AppendObjectFile(context.Background())
		if err != nil {
			t.Fatalf("open stale append stream: %v", err)
		}
		if err := stream.Send(&sharedv1.AppendFileChunk{
			ObjectId:            "obj_001",
			Filename:            "stale.txt",
			Data:                []byte("!"),
			FinalChunk:          true,
			CurrentExpectedSize: 4,
			ExpectedSize:        6,
		}); err != nil {
			t.Fatalf("send stale append: %v", err)
		}
		if _, err := stream.CloseAndRecv(); status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("stale append correct size succeeds", func(t *testing.T) {
		if _, err := svc.WriteObjectFile(context.Background(), "obj_001", "fresh.txt", []byte("hello")); err != nil {
			t.Fatalf("seed fresh file: %v", err)
		}
		stream, err := client.AppendObjectFile(context.Background())
		if err != nil {
			t.Fatalf("open fresh append stream: %v", err)
		}
		if err := stream.Send(&sharedv1.AppendFileChunk{
			ObjectId:            "obj_001",
			Filename:            "fresh.txt",
			Data:                []byte("!"),
			FinalChunk:          true,
			CurrentExpectedSize: 5,
			ExpectedSize:        6,
		}); err != nil {
			t.Fatalf("send fresh append: %v", err)
		}
		if _, err := stream.CloseAndRecv(); err != nil {
			t.Fatalf("close fresh append: %v", err)
		}
	})

	t.Run("oversized upload fails mid stream", func(t *testing.T) {
		stream, err := client.WriteObjectFile(context.Background())
		if err != nil {
			t.Fatalf("open oversize stream: %v", err)
		}
		if err := stream.Send(&sharedv1.WriteFileChunk{
			ObjectId:     "obj_001",
			Filename:     "oversize.bin",
			Data:         bytes.Repeat([]byte("a"), MAX_OBJECT_FILE_BYTES-1),
			FinalChunk:   false,
			ExpectedSize: int64(MAX_OBJECT_FILE_BYTES + 1),
		}); err != nil {
			t.Fatalf("send first oversize chunk: %v", err)
		}
		if err := stream.Send(&sharedv1.WriteFileChunk{
			ObjectId:     "obj_001",
			Filename:     "oversize.bin",
			Data:         []byte("bc"),
			FinalChunk:   true,
			ExpectedSize: int64(MAX_OBJECT_FILE_BYTES + 1),
		}); err != nil {
			t.Fatalf("send second oversize chunk: %v", err)
		}
		if _, err := stream.CloseAndRecv(); status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("expected ResourceExhausted, got %v", err)
		}
	})

	t.Run("chunk metadata mismatch fails", func(t *testing.T) {
		stream, err := client.WriteObjectFile(context.Background())
		if err != nil {
			t.Fatalf("open mismatch stream: %v", err)
		}
		if err := stream.Send(&sharedv1.WriteFileChunk{
			ObjectId:     "obj_001",
			Filename:     "a.txt",
			Data:         []byte("a"),
			FinalChunk:   false,
			ExpectedSize: 2,
		}); err != nil {
			t.Fatalf("send first mismatch chunk: %v", err)
		}
		if err := stream.Send(&sharedv1.WriteFileChunk{
			ObjectId:     "obj_001",
			Filename:     "b.txt",
			Data:         []byte("b"),
			FinalChunk:   true,
			ExpectedSize: 2,
		}); err != nil {
			t.Fatalf("send second mismatch chunk: %v", err)
		}
		if _, err := stream.CloseAndRecv(); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("empty file write succeeds", func(t *testing.T) {
		stream, err := client.WriteObjectFile(context.Background())
		if err != nil {
			t.Fatalf("open empty write stream: %v", err)
		}
		if err := stream.Send(&sharedv1.WriteFileChunk{
			ObjectId:     "obj_001",
			Filename:     "empty.txt",
			FinalChunk:   true,
			ExpectedSize: 0,
		}); err != nil {
			t.Fatalf("send empty write chunk: %v", err)
		}
		if _, err := stream.CloseAndRecv(); err != nil {
			t.Fatalf("close empty write stream: %v", err)
		}
		manifest, err := svc.GetObjectManifest(context.Background(), "obj_001")
		if err != nil {
			t.Fatalf("get manifest: %v", err)
		}
		file, ok := manifest.Files["empty.txt"]
		if !ok {
			t.Fatal("expected empty.txt to be present in manifest")
		}
		if file.Size != 0 {
			t.Fatalf("expected empty.txt size 0, got %d", file.Size)
		}
	})

	t.Run("client disconnect cleans partial write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stream, err := client.WriteObjectFile(ctx)
		if err != nil {
			t.Fatalf("open disconnect stream: %v", err)
		}
		if err := stream.Send(&sharedv1.WriteFileChunk{
			ObjectId:     "obj_001",
			Filename:     "partial.txt",
			Data:         []byte("partial"),
			FinalChunk:   false,
			ExpectedSize: 14,
		}); err != nil {
			t.Fatalf("send disconnect chunk: %v", err)
		}
		cancel()
		_ = stream.CloseSend()
		waitForPartialUploadCleanup(t, svc, "obj_001", "partial.txt")
	})
}

func waitForPartialUploadCleanup(t *testing.T, svc *Service, objectID, filename string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		manifest, err := svc.GetObjectManifest(context.Background(), objectID)
		if err != nil {
			t.Fatalf("get manifest after disconnect: %v", err)
		}
		files, err := svc.objectStorage.ListObjectFolderFiles(objectID)
		if err != nil {
			t.Fatalf("list object files after disconnect: %v", err)
		}
		_, inManifest := manifest.Files[filename]
		inFiles := false
		for _, file := range files {
			if file == filename {
				inFiles = true
				break
			}
		}
		if !inManifest && !inFiles {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s cleanup", filename)
}
