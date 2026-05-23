package function

import (
	"atlas.local/protocol"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/functions/internal/gateway"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

// testObservationJSON is minimal valid observation JSON for tests (identity present).
var testObservationJSON = []byte(`{"identity":{"kind":"asset"}}`)

func testLogger() *logging.Logger {
	return logging.New("debug", "atlas-test", "test")
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed.UTC()
}

func testProtoValidator() *protocolvalidation.Validator {
	v, err := protocolvalidation.New()
	if err != nil {
		panic(fmt.Sprintf("init protocol validator: %v", err))
	}
	return v
}

type fakeProtocolValidator struct {
	entityIssues             []protocol.ValidationIssue
	objectIssues             []protocol.ValidationIssue
	taskIssues               []protocol.ValidationIssue
	observationIssues        []protocol.ValidationIssue
	historyEventIssues       []protocol.ValidationIssue
	commandCatalogJSONIssues []protocol.ValidationIssue
}

func (f fakeProtocolValidator) ValidateEntity(entity *model.Entity) []protocol.ValidationIssue {
	return f.entityIssues
}

func (f fakeProtocolValidator) ValidateObject(obj *model.Object) []protocol.ValidationIssue {
	return f.objectIssues
}

func (f fakeProtocolValidator) ValidateTask(task *model.Task) []protocol.ValidationIssue {
	return f.taskIssues
}

func (f fakeProtocolValidator) ValidateObservation(obs *model.Observation) []protocol.ValidationIssue {
	return f.observationIssues
}

func (f fakeProtocolValidator) ValidateObservationHistoryEvent([]byte) []protocol.ValidationIssue {
	return f.historyEventIssues
}

func (f fakeProtocolValidator) ValidateCommandCatalogJSON(data []byte) []protocol.ValidationIssue {
	return f.commandCatalogJSONIssues
}

type fakeEntityStore struct {
	getFn func(context.Context, string) (*model.Entity, error)
}

func (s *fakeEntityStore) CreateEntity(context.Context, *model.Entity) error { return nil }
func (s *fakeEntityStore) GetEntity(ctx context.Context, id string) (*model.Entity, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return nil, model.ErrNotFound
}
func (s *fakeEntityStore) ListEntities(context.Context, store.EntityListParams) (store.EntityListResult, error) {
	return store.EntityListResult{}, nil
}
func (s *fakeEntityStore) UpdateEntity(context.Context, *model.Entity) error { return nil }
func (s *fakeEntityStore) DeleteEntity(context.Context, string) error        { return nil }
func (s *fakeEntityStore) UpsertEntity(context.Context, *model.Entity) error { return nil }

type fakeTaskStore struct {
	createFn func(context.Context, *model.Task) error
	getFn    func(context.Context, string) (*model.Task, error)
	updateFn func(context.Context, *model.Task) error
	deleteFn func(context.Context, string) error
	upsertFn func(context.Context, *model.Task) error
}

func (s fakeTaskStore) CreateTask(ctx context.Context, task *model.Task) error {
	if s.createFn != nil {
		return s.createFn(ctx, task)
	}
	return nil
}
func (s fakeTaskStore) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	if s.getFn != nil {
		return s.getFn(ctx, taskID)
	}
	return nil, model.ErrNotFound
}
func (s fakeTaskStore) ListTasks(context.Context, store.TaskListParams) (store.TaskListResult, error) {
	return store.TaskListResult{}, nil
}
func (s fakeTaskStore) UpdateTask(ctx context.Context, task *model.Task) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, task)
	}
	return nil
}
func (s fakeTaskStore) DeleteTask(ctx context.Context, taskID string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, taskID)
	}
	return nil
}
func (s fakeTaskStore) UpsertTask(ctx context.Context, task *model.Task) error {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, task)
	}
	return nil
}

type captureObservationStore struct {
	created *model.Observation
	updated *model.Observation
	upsert  *model.Observation
	byID    map[string]*model.Observation
}

func (s *captureObservationStore) CreateObservation(_ context.Context, obs *model.Observation) error {
	cp := *obs
	cp.Version = 1
	s.created = &cp
	if s.byID == nil {
		s.byID = map[string]*model.Observation{}
	}
	stored := cp
	s.byID[obs.ObservationID] = &stored
	return nil
}
func (s *captureObservationStore) GetObservation(_ context.Context, observationID string) (*model.Observation, error) {
	if s.byID != nil {
		if obs, ok := s.byID[observationID]; ok {
			cp := *obs
			return &cp, nil
		}
	}
	return nil, model.ErrNotFound
}
func (s *captureObservationStore) ListObservations(context.Context, store.ObservationListParams) (store.ObservationListResult, error) {
	return store.ObservationListResult{}, nil
}
func (s *captureObservationStore) UpdateObservation(_ context.Context, obs *model.Observation) error {
	cp := *obs
	cp.Version++
	s.updated = &cp
	if s.byID == nil {
		s.byID = map[string]*model.Observation{}
	}
	stored := cp
	s.byID[obs.ObservationID] = &stored
	return nil
}
func (s *captureObservationStore) DeleteObservation(context.Context, string) error {
	return nil
}
func (s *captureObservationStore) UpsertObservation(_ context.Context, obs *model.Observation) error {
	cp := *obs
	s.upsert = &cp
	if s.byID == nil {
		s.byID = map[string]*model.Observation{}
	}
	stored := cp
	s.byID[obs.ObservationID] = &stored
	return nil
}

type fakeObjectStore struct {
	createFn             func(context.Context, *model.Object) error
	getFn                func(context.Context, string) (*model.Object, error)
	listFn               func(context.Context, store.ObjectListParams) (store.ObjectListResult, error)
	updateFn             func(context.Context, *model.Object) error
	deleteFn             func(context.Context, string) error
	upsertFn             func(context.Context, *model.Object) error
	updateManifestFn     func(context.Context, string, *model.ObjectManifest, ...time.Time) error
	getManifestFn        func(context.Context, string) (*model.ObjectManifest, error)
	updatedManifestCalls int
}

func (s *fakeObjectStore) CreateObject(ctx context.Context, obj *model.Object) error {
	if s.createFn != nil {
		return s.createFn(ctx, obj)
	}
	return nil
}
func (s *fakeObjectStore) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	if s.getFn != nil {
		return s.getFn(ctx, objectID)
	}
	return nil, model.ErrNotFound
}
func (s *fakeObjectStore) ListObjects(ctx context.Context, params store.ObjectListParams) (store.ObjectListResult, error) {
	if s.listFn != nil {
		return s.listFn(ctx, params)
	}
	return store.ObjectListResult{}, nil
}
func (s *fakeObjectStore) UpdateObject(ctx context.Context, obj *model.Object) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, obj)
	}
	return nil
}
func (s *fakeObjectStore) DeleteObject(ctx context.Context, objectID string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, objectID)
	}
	return nil
}
func (s *fakeObjectStore) UpsertObject(ctx context.Context, obj *model.Object) error {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, obj)
	}
	return nil
}
func (s *fakeObjectStore) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest, updatedAt ...time.Time) error {
	s.updatedManifestCalls++
	if s.updateManifestFn != nil {
		return s.updateManifestFn(ctx, objectID, manifest, updatedAt...)
	}
	return nil
}
func (s *fakeObjectStore) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	if s.getManifestFn != nil {
		return s.getManifestFn(ctx, objectID)
	}
	return model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}), nil
}

type fakeObjectGateway struct {
	store.ObjectStore
	appended []objectAppendCall
	files    map[string]map[string][]byte
}

type objectAppendCall struct {
	objectID string
	filename string
	data     []byte
}

func countAppendedFilename(calls []objectAppendCall, filename string) int {
	n := 0
	for _, c := range calls {
		if c.filename == filename {
			n++
		}
	}
	return n
}

func (g *fakeObjectGateway) EnsureObjectCreated(ctx context.Context, obj *model.Object) error {
	if existing, err := g.GetObject(ctx, obj.ObjectID); err == nil {
		*obj = *existing
		return nil
	} else if !errors.Is(err, model.ErrNotFound) {
		return err
	}
	if err := g.CreateObject(ctx, obj); err != nil {
		return err
	}
	if stored, err := g.GetObject(ctx, obj.ObjectID); err == nil {
		*obj = *stored
		return nil
	} else if !errors.Is(err, model.ErrNotFound) {
		return err
	}
	return nil
}

func (g *fakeObjectGateway) WriteFile(ctx context.Context, objectID, filename string, data []byte) (gateway.ManifestResult, error) {
	if _, err := g.GetObject(ctx, objectID); err != nil {
		return gateway.ManifestResult{}, err
	}
	if g.files == nil {
		g.files = map[string]map[string][]byte{}
	}
	if g.files[objectID] == nil {
		g.files[objectID] = map[string][]byte{}
	}
	g.files[objectID][filename] = append([]byte(nil), data...)
	return gateway.ManifestResult{
		Manifest:        model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}),
		ManifestCurrent: true,
	}, nil
}

func (g *fakeObjectGateway) AppendFile(ctx context.Context, objectID, filename string, data []byte) (gateway.ManifestResult, error) {
	if _, err := g.GetObject(ctx, objectID); err != nil {
		return gateway.ManifestResult{}, err
	}
	g.appended = append(g.appended, objectAppendCall{
		objectID: objectID,
		filename: filename,
		data:     append([]byte(nil), data...),
	})
	if g.files == nil {
		g.files = map[string]map[string][]byte{}
	}
	if g.files[objectID] == nil {
		g.files[objectID] = map[string][]byte{}
	}
	g.files[objectID][filename] = append(g.files[objectID][filename], data...)
	return gateway.ManifestResult{
		Manifest:        model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}),
		ManifestCurrent: true,
	}, nil
}

func (g *fakeObjectGateway) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	if _, err := g.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	if g.files != nil && g.files[objectID] != nil {
		return append([]byte(nil), g.files[objectID][filename]...), nil
	}
	return nil, nil
}

func (g *fakeObjectGateway) DeleteFile(ctx context.Context, objectID, filename string) (gateway.ManifestResult, error) {
	if _, err := g.GetObject(ctx, objectID); err != nil {
		return gateway.ManifestResult{}, err
	}
	if g.files != nil && g.files[objectID] != nil {
		delete(g.files[objectID], filename)
	}
	return gateway.ManifestResult{
		Manifest:        model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}),
		ManifestCurrent: true,
	}, nil
}

func (g *fakeObjectGateway) ListFiles(ctx context.Context, objectID string) ([]string, error) {
	if _, err := g.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	return nil, nil
}

func (g *fakeObjectGateway) Reconcile(context.Context) error {
	return nil
}

func newTestObjectFunctions(metadata store.ObjectStore, idem store.IdempotencyStore, log *logging.Logger, protoValidator ProtocolValidator, publishers ...Publisher) ObjectFunctions {
	return NewObjectFunctions(&fakeObjectGateway{ObjectStore: metadata}, idem, log, protoValidator, publishers...)
}

type fakeIdempotencyStore struct {
	tryBeginFn      func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error)
	markCompletedFn func(context.Context, string, string) error
	markFailedFn    func(context.Context, string, string) error
}

type capturePublisher struct {
	events []*sharedv1.MutationEvent
}

func (p *capturePublisher) Publish(_ context.Context, event *sharedv1.MutationEvent) {
	p.events = append(p.events, event)
}

func TestFakeObjectGatewayAppendFilePreservesContent(t *testing.T) {
	obj := &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeObservationHistory}
	objectStore := &fakeObjectStore{
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if objectID == obj.ObjectID {
				return obj, nil
			}
			return nil, model.ErrNotFound
		},
	}
	gw := &fakeObjectGateway{ObjectStore: objectStore}
	ctx := context.Background()
	if _, err := gw.WriteFile(ctx, obj.ObjectID, "data.txt", []byte("a")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := gw.AppendFile(ctx, obj.ObjectID, "data.txt", []byte("b")); err != nil {
		t.Fatalf("AppendFile: %v", err)
	}
	got, err := gw.ReadFile(ctx, obj.ObjectID, "data.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "ab" {
		t.Fatalf("ReadFile = %q, want %q", got, "ab")
	}
}

func TestFakeObjectGatewayDeleteFileRemovesEntry(t *testing.T) {
	obj := &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog}
	objectStore := &fakeObjectStore{
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if objectID == obj.ObjectID {
				return obj, nil
			}
			return nil, model.ErrNotFound
		},
	}
	gw := &fakeObjectGateway{
		ObjectStore: objectStore,
		files:       map[string]map[string][]byte{"obj_001": {"data.txt": []byte("payload")}},
	}
	ctx := context.Background()
	if _, err := gw.DeleteFile(ctx, obj.ObjectID, "data.txt"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if _, ok := gw.files[obj.ObjectID]["data.txt"]; ok {
		t.Fatal("expected file entry to be removed")
	}
	got, err := gw.ReadFile(ctx, obj.ObjectID, "data.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty read after delete, got %q", got)
	}
}
