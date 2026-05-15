//go:build integration
// +build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	functionsAddr = "127.0.0.1:8080"
	filename      = "test.txt"
)

func TestCrossServiceEndToEnd(t *testing.T) {
	composeUp(t)
	t.Cleanup(func() {
		composeDown(t)
	})

	waitForReady(t, functionsAddr)

	conn := dialGRPC(t, functionsAddr)
	defer conn.Close()

	client := functionsv1.NewAtlasFunctionsServiceClient(conn)
	changefeedClient := functionsv1.NewChangefeedServiceClient(conn)

	entity := createTestEntity(t, client)
	gotEntity := getEntity(t, client, entity.GetEntityId())
	assertEntityEqual(t, entity, gotEntity)
	entity = gotEntity

	stream := subscribeMutations(t, changefeedClient)
	entity2 := createTestEntity(t, client)
	event := recvMutation(t, stream)
	if event.GetResource() != "entity" {
		t.Fatalf("expected entity mutation resource, got %q", event.GetResource())
	}
	if event.GetOperation() != "created" {
		t.Fatalf("expected created mutation, got %q", event.GetOperation())
	}
	if event.GetResourceId() != entity2.GetEntityId() {
		t.Fatalf("expected mutation for %q, got %q", entity2.GetEntityId(), event.GetResourceId())
	}
	assertEntityEqual(t, entity2, event.GetEntity())

	object := createTestObject(t, client)
	gotObject := getObject(t, client, object.GetObjectId())
	assertObjectEqual(t, object, gotObject)
	object = gotObject

	writeFile(t, client, object.GetObjectId(), filename, []byte("hello integration"))
	data := readFile(t, client, object.GetObjectId(), filename)
	assertBytesEqual(t, []byte("hello integration"), data)

	appendFile(t, client, object.GetObjectId(), filename, []byte(" world"))
	data = readFile(t, client, object.GetObjectId(), filename)
	assertBytesEqual(t, []byte("hello integration world"), data)
	object = getObject(t, client, object.GetObjectId())

	appendErr := appendFileExpectError(t, client, object.GetObjectId(), filename, []byte("bad"), 0)
	if status.Code(appendErr) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted for stale append, got %v (%v)", status.Code(appendErr), appendErr)
	}
	if !strings.Contains(appendErr.Error(), "current_expected_size") {
		t.Fatalf("expected stale append error to mention current_expected_size, got %v", appendErr)
	}

	restartFunctionsContainer(t)
	conn.Close()

	waitForReady(t, functionsAddr)
	conn = dialGRPC(t, functionsAddr)
	defer conn.Close()
	client = functionsv1.NewAtlasFunctionsServiceClient(conn)

	gotAfterRestart := getEntity(t, client, entity.GetEntityId())
	assertEntityEqual(t, entity, gotAfterRestart)
	gotObjectAfterRestart := getObject(t, client, object.GetObjectId())
	assertObjectEqual(t, object, gotObjectAfterRestart)
	dataAfterRestart := readFile(t, client, object.GetObjectId(), filename)
	assertBytesEqual(t, []byte("hello integration world"), dataAfterRestart)
}

func dialGRPC(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial grpc %s: %v", addr, err)
	}
	return conn
}

func createTestEntity(t *testing.T, client functionsv1.AtlasFunctionsServiceClient) *sharedv1.Entity {
	t.Helper()
	resp, err := client.CreateEntity(context.Background(), &sharedv1.EntityRequest{
		Entity: &sharedv1.Entity{
			EntityId: uniqueID("asset"),
			Type:     "asset",
			Json:     []byte(`{"components":{"supported_commands":{"commands":["integration_cmd"]}}}`),
		},
	})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	return resp.GetEntity()
}

func getEntity(t *testing.T, client functionsv1.AtlasFunctionsServiceClient, entityID string) *sharedv1.Entity {
	t.Helper()
	resp, err := client.GetEntity(context.Background(), &sharedv1.GetEntityRequest{EntityId: entityID})
	if err != nil {
		t.Fatalf("get entity %s: %v", entityID, err)
	}
	return resp.GetEntity()
}

func subscribeMutations(t *testing.T, client functionsv1.ChangefeedServiceClient) functionsv1.ChangefeedService_SubscribeMutationsClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	stream, err := client.SubscribeMutations(ctx, &functionsv1.SubscribeMutationsRequest{})
	if err != nil {
		t.Fatalf("subscribe mutations: %v", err)
	}
	return stream
}

func recvMutation(t *testing.T, stream functionsv1.ChangefeedService_SubscribeMutationsClient) *sharedv1.MutationEvent {
	t.Helper()
	event, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv mutation: %v", err)
	}
	return event
}

func createTestObject(t *testing.T, client functionsv1.AtlasFunctionsServiceClient) *sharedv1.Object {
	t.Helper()
	resp, err := client.CreateObject(context.Background(), &sharedv1.ObjectRequest{
		Object: &sharedv1.Object{
			ObjectId:  uniqueID("obj"),
			Type:      "log",
			OwnerType: "system",
			OwnerId:   "system",
			Json:      []byte(`{}`),
		},
	})
	if err != nil {
		t.Fatalf("create object: %v", err)
	}
	return resp.GetObject()
}

func getObject(t *testing.T, client functionsv1.AtlasFunctionsServiceClient, objectID string) *sharedv1.Object {
	t.Helper()
	resp, err := client.GetObject(context.Background(), &sharedv1.GetObjectRequest{ObjectId: objectID})
	if err != nil {
		t.Fatalf("get object %s: %v", objectID, err)
	}
	return resp.GetObject()
}

func writeFile(t *testing.T, client functionsv1.AtlasFunctionsServiceClient, objectID, name string, data []byte) {
	t.Helper()
	_, err := client.WriteObjectFile(context.Background(), &sharedv1.WriteObjectFileRequest{
		ObjectId: objectID,
		Filename: name,
		Data:     data,
	})
	if err != nil {
		t.Fatalf("write object file %s/%s: %v", objectID, name, err)
	}
}

func readFile(t *testing.T, client functionsv1.AtlasFunctionsServiceClient, objectID, name string) []byte {
	t.Helper()
	resp, err := client.ReadObjectFile(context.Background(), &sharedv1.ReadObjectFileRequest{
		ObjectId: objectID,
		Filename: name,
	})
	if err != nil {
		t.Fatalf("read object file %s/%s: %v", objectID, name, err)
	}
	return resp.GetData()
}

func appendFile(t *testing.T, client functionsv1.AtlasFunctionsServiceClient, objectID, name string, data []byte) {
	t.Helper()
	_, err := client.AppendObjectFile(context.Background(), &sharedv1.WriteObjectFileRequest{
		ObjectId: objectID,
		Filename: name,
		Data:     data,
	})
	if err != nil {
		t.Fatalf("append object file %s/%s: %v", objectID, name, err)
	}
}

func appendFileExpectError(t *testing.T, client functionsv1.AtlasFunctionsServiceClient, objectID, name string, data []byte, expectedSize int64) error {
	t.Helper()
	req := &sharedv1.WriteObjectFileRequest{
		ObjectId: objectID,
		Filename: name,
		Data:     data,
	}
	req.CurrentExpectedSize = &expectedSize
	_, err := client.AppendObjectFile(context.Background(), req)
	if err == nil {
		t.Fatalf("expected append object file %s/%s to fail", objectID, name)
	}
	return err
}

func restartFunctionsContainer(t *testing.T) {
	t.Helper()
	runCompose(t, 30*time.Second, "restart", "atlas-functions")
}

func waitForReady(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		conn, err := grpc.DialContext(
			ctx,
			addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		cancel()
		if err == nil {
			conn.Close()
			return
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("wait for ready %s: %v", addr, lastErr)
}

func composeUp(t *testing.T) {
	t.Helper()
	runCompose(t, 90*time.Second, "up", "-d", "--wait")
}

func composeDown(t *testing.T) {
	t.Helper()
	runCompose(t, 60*time.Second, "down", "-v", "--remove-orphans")
}

func runCompose(t *testing.T, timeout time.Duration, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"docker",
		append([]string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.integration.yml"}, args...)...,
	)
	cmd.Dir = atlasCoreDir(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func atlasCoreDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("determine integration test path")
	}
	return filepath.Dir(filepath.Dir(file))
}

func uniqueID(prefix string) string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(fmt.Sprintf("rand read: %v", err))
	}
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), hex.EncodeToString(buf[:]))
}

func assertEntityEqual(t *testing.T, want, got *sharedv1.Entity) {
	t.Helper()
	if want.GetEntityId() != got.GetEntityId() || want.GetType() != got.GetType() || want.GetSubtype() != got.GetSubtype() || want.GetAlias() != got.GetAlias() || want.GetVersion() != got.GetVersion() {
		t.Fatalf("entity mismatch\nwant: %s\ngot:  %s", want, got)
	}
	assertJSONEqual(t, want.GetJson(), got.GetJson())
	assertTimestampPresent(t, "entity.created_at", got.GetCreatedAt())
	assertTimestampPresent(t, "entity.updated_at", got.GetUpdatedAt())
}

func assertObjectEqual(t *testing.T, want, got *sharedv1.Object) {
	t.Helper()
	if want.GetObjectId() != got.GetObjectId() || want.GetType() != got.GetType() || want.GetOwnerType() != got.GetOwnerType() || want.GetOwnerId() != got.GetOwnerId() || want.GetVersion() != got.GetVersion() {
		t.Fatalf("object mismatch\nwant: %s\ngot:  %s", want, got)
	}
	assertJSONEqual(t, want.GetJson(), got.GetJson())
	assertTimestampPresent(t, "object.created_at", got.GetCreatedAt())
	assertTimestampPresent(t, "object.updated_at", got.GetUpdatedAt())
}

func assertBytesEqual(t *testing.T, want, got []byte) {
	t.Helper()
	if string(want) != string(got) {
		t.Fatalf("bytes mismatch\nwant: %q\ngot:  %q", string(want), string(got))
	}
}

func assertJSONEqual(t *testing.T, want, got []byte) {
	t.Helper()
	var wantJSON any
	if err := json.Unmarshal(want, &wantJSON); err != nil {
		t.Fatalf("unmarshal want json: %v", err)
	}
	var gotJSON any
	if err := json.Unmarshal(got, &gotJSON); err != nil {
		t.Fatalf("unmarshal got json: %v", err)
	}
	if !reflect.DeepEqual(wantJSON, gotJSON) {
		t.Fatalf("json mismatch\nwant: %s\ngot:  %s", string(want), string(got))
	}
}

func assertTimestampPresent(t *testing.T, name string, ts interface {
	IsValid() bool
	AsTime() time.Time
}) {
	t.Helper()
	if ts == nil || !ts.IsValid() || ts.AsTime().IsZero() {
		t.Fatalf("%s must be present and valid", name)
	}
}
