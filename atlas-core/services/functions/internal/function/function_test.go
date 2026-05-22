package function

import (
	"atlas.local/protocol"
	"bytes"
	"context"
	"encoding/json"
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
	return nil
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
	return gateway.ManifestResult{
		Manifest:        model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}),
		ManifestCurrent: true,
	}, nil
}

func (g *fakeObjectGateway) AppendFile(ctx context.Context, objectID, filename string, data []byte) (gateway.ManifestResult, error) {
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
	return g.WriteFile(ctx, objectID, filename, data)
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
	return g.WriteFile(ctx, objectID, filename, nil)
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

func TestPublisherOrNopUsesFirstNonNilPublisher(t *testing.T) {
	first := &capturePublisher{}
	if got := publisherOrNop([]Publisher{nil, first}); got != first {
		t.Fatalf("expected first non-nil publisher, got %T", got)
	}
}

func (s fakeIdempotencyStore) TryBegin(ctx context.Context, scope, key, resourceID string) (store.IdempotencyRecord, bool, error) {
	if s.tryBeginFn != nil {
		return s.tryBeginFn(ctx, scope, key, resourceID)
	}
	return store.IdempotencyRecord{ResourceID: resourceID, Status: store.IdempotencyStatusPending}, true, nil
}

func (s fakeIdempotencyStore) MarkCompleted(ctx context.Context, scope, key string) error {
	if s.markCompletedFn != nil {
		return s.markCompletedFn(ctx, scope, key)
	}
	return nil
}

func (s fakeIdempotencyStore) MarkFailed(ctx context.Context, scope, key string) error {
	if s.markFailedFn != nil {
		return s.markFailedFn(ctx, scope, key)
	}
	return nil
}

func TestEntityFunctions_ValidateEntityID(t *testing.T) {
	f := EntityFunctions{}
	entity := &model.Entity{Type: model.EntityTypeAsset, JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateEntity(nil, entity); err == nil {
		t.Fatal("expected error for empty entity_id")
	}
	entity.EntityID = "this-entity-id-is-way-too-long-for-the-50-character-limit"
	if err := f.CreateEntity(nil, entity); err == nil {
		t.Fatal("expected error for long entity_id")
	}
}

func TestEntityFunctions_ValidateType(t *testing.T) {
	f := EntityFunctions{}
	entity := &model.Entity{EntityID: "test_001", Type: model.EntityType("invalid_type"), JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateEntity(nil, entity); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestObservationFunctions_ValidateObservationID(t *testing.T) {
	f := ObservationFunctions{}
	obs := &model.Observation{SourceAssetID: "asset_001", JSON: minimumObservationJSON, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObservation(nil, obs); err == nil {
		t.Fatal("expected error for empty observation_id")
	}
}

func TestObservationFunctions_ValidateSourceAssetID(t *testing.T) {
	f := ObservationFunctions{}
	obs := &model.Observation{ObservationID: "obs_001", JSON: minimumObservationJSON, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObservation(nil, obs); err == nil {
		t.Fatal("expected error for empty source_asset_id")
	}
}

func TestValidateObservationJSON_RejectsWhitespaceEmptyObject(t *testing.T) {
	for _, json := range [][]byte{
		[]byte(`{}`),
		[]byte(`{ }`),
		[]byte("{\n}"),
		[]byte(`  {  }  `),
	} {
		if err := validateObservationJSON(json); err == nil {
			t.Fatalf("expected error for empty json object %q", json)
		} else if fieldErr, ok := err.(*model.FieldError); !ok || fieldErr.Field != "json" {
			t.Fatalf("expected field error on json for %q, got %T: %v", json, err, err)
		}
	}
	if err := validateObservationJSON(minimumObservationJSON); err != nil {
		t.Fatalf("expected minimum observation json to be valid, got %v", err)
	}
}

func TestObservationFunctions_CreateObservationRequiresStartedAt(t *testing.T) {
	store := &captureObservationStore{}
	f := NewObservationFunctions(store, testLogger(), fakeProtocolValidator{})
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		JSON:          minimumObservationJSON,
	}
	if err := f.CreateObservation(context.Background(), obs); err == nil {
		t.Fatal("expected error for missing started_at")
	}
}

func TestObservationFunctions_CreateObservationRejectsLatestTelemetry(t *testing.T) {
	store := &captureObservationStore{}
	f := NewObservationFunctions(store, testLogger(), fakeProtocolValidator{})
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     mustParseTime(t, "2026-01-01T00:00:00Z"),
		JSON:          []byte(`{"latest_telemetry":{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}}`),
	}
	if err := f.CreateObservation(context.Background(), obs); err == nil {
		t.Fatal("expected error for latest_telemetry on create")
	}
}

func TestObservationFunctions_CreateObservationPersistsIdentitySideEffects(t *testing.T) {
	obsStore := &captureObservationStore{}
	var createdObject *model.Object
	objectStore := &fakeObjectStore{
		createFn: func(_ context.Context, obj *model.Object) error {
			cp := *obj
			createdObject = &cp
			return nil
		},
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if createdObject != nil && createdObject.ObjectID == objectID {
				return createdObject, nil
			}
			return nil, model.ErrNotFound
		},
	}
	objectGateway := &fakeObjectGateway{ObjectStore: objectStore}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator()).
		WithObjectGateway(objectGateway)

	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     startedAt,
		JSON:          []byte(`{"identity":{"kind":"vehicle"}}`),
	}
	if err := f.CreateObservation(context.Background(), obs); err != nil {
		t.Fatalf("CreateObservation failed: %v", err)
	}
	if obsStore.created == nil {
		t.Fatal("expected initial create")
	}
	if observationJSONHasKey(obsStore.created.JSON, "history_object_id") {
		t.Fatalf("expected initial create without history_object_id, got %s", string(obsStore.created.JSON))
	}
	if obsStore.updated == nil {
		t.Fatal("expected follow-up update persisting identity side effects")
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"history_object_id"`)) {
		t.Fatalf("expected persisted JSON to include history_object_id, got %s", string(obsStore.updated.JSON))
	}
	if obsStore.updated.LatestIdentityAt == nil || !obsStore.updated.LatestIdentityAt.Equal(startedAt) {
		t.Fatalf("expected latest_identity_at %v, got %v", startedAt, obsStore.updated.LatestIdentityAt)
	}
	if len(objectGateway.appended) != 1 {
		t.Fatalf("expected one identity_patch append, got %d", len(objectGateway.appended))
	}
}

func TestObservationFunctions_UpdateObservationPersistsIdentitySideEffects(t *testing.T) {
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID: "obs_001",
				SourceAssetID: "asset_001",
				StartedAt:     mustParseTime(t, "2026-01-01T00:00:00Z"),
				Version:       1,
				JSON:          minimumObservationJSON,
			},
		},
	}
	var createdObject *model.Object
	objectStore := &fakeObjectStore{
		createFn: func(_ context.Context, obj *model.Object) error {
			cp := *obj
			createdObject = &cp
			return nil
		},
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if createdObject != nil && createdObject.ObjectID == objectID {
				return createdObject, nil
			}
			return nil, model.ErrNotFound
		},
	}
	objectGateway := &fakeObjectGateway{ObjectStore: objectStore}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator()).
		WithObjectGateway(objectGateway)

	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     startedAt,
		Version:       1,
		JSON:          []byte(`{"identity":{"kind":"vehicle"}}`),
	}
	if err := f.UpdateObservation(context.Background(), obs); err != nil {
		t.Fatalf("UpdateObservation failed: %v", err)
	}
	if obsStore.updated == nil {
		t.Fatal("expected observation update")
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"history_object_id"`)) {
		t.Fatalf("expected persisted JSON to include history_object_id, got %s", string(obsStore.updated.JSON))
	}
	if obsStore.updated.LatestIdentityAt == nil {
		t.Fatal("expected latest_identity_at to be persisted")
	}
	if len(objectGateway.appended) != 1 {
		t.Fatalf("expected one identity_patch append, got %d", len(objectGateway.appended))
	}
}

func TestObservationFunctions_UpsertObservationRejectsLatestTelemetry(t *testing.T) {
	store := &captureObservationStore{}
	f := NewObservationFunctions(store, testLogger(), fakeProtocolValidator{})
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     mustParseTime(t, "2026-01-01T00:00:00Z"),
		JSON:          []byte(`{"latest_telemetry":{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}}`),
	}
	err := f.UpsertObservation(context.Background(), obs)
	if err == nil {
		t.Fatal("expected error for latest_telemetry on upsert")
	}
	fieldErr, ok := err.(*model.FieldError)
	if !ok {
		t.Fatalf("expected FieldError, got %T: %v", err, err)
	}
	if fieldErr.Field != "json.latest_telemetry" {
		t.Fatalf("expected field json.latest_telemetry, got %q", fieldErr.Field)
	}
}

func TestObservationFunctions_IngestObservationTelemetryArchivesAndUpdatesCurrentState(t *testing.T) {
	obsStore := &captureObservationStore{}
	var createdObject *model.Object
	objectStore := &fakeObjectStore{
		createFn: func(_ context.Context, obj *model.Object) error {
			cp := *obj
			createdObject = &cp
			return nil
		},
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if createdObject != nil && createdObject.ObjectID == objectID {
				return createdObject, nil
			}
			return nil, model.ErrNotFound
		},
	}
	objectGateway := &fakeObjectGateway{ObjectStore: objectStore}
	entityStore := &fakeEntityStore{
		getFn: func(_ context.Context, id string) (*model.Entity, error) {
			switch id {
			case "asset_001":
				return &model.Entity{EntityID: id, Type: model.EntityTypeAsset}, nil
			case "track_001":
				return &model.Entity{EntityID: id, Type: model.EntityTypeTrack}, nil
			default:
				return nil, model.ErrNotFound
			}
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator()).
		WithObjectGateway(objectGateway).
		WithEntityStore(entityStore)

	targetEntityID := "track_001"
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obs, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID:  "obs_001",
		SourceAssetID:  "asset_001",
		TargetEntityID: &targetEntityID,
		TelemetryJSON:  []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
		StartedAt:      startedAt,
	})
	if err != nil {
		t.Fatalf("IngestObservationTelemetry failed: %v", err)
	}

	historyObjectID := ObservationHistoryObjectID("obs_001")
	if createdObject == nil {
		t.Fatal("expected history object to be created")
	}
	if createdObject.ObjectID != historyObjectID || createdObject.Type != model.ObjectTypeObservationHistory {
		t.Fatalf("unexpected history object: %+v", createdObject)
	}
	if len(objectGateway.appended) != 1 {
		t.Fatalf("expected one history append, got %d", len(objectGateway.appended))
	}
	appendCall := objectGateway.appended[0]
	if appendCall.objectID != historyObjectID || appendCall.filename != ObservationHistoryFilename {
		t.Fatalf("unexpected append target: %+v", appendCall)
	}
	var appendedEvent map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(appendCall.data), &appendedEvent); err != nil {
		t.Fatalf("failed to unmarshal appended history event: %v", err)
	}
	if appendedEvent["event_type"] != "telemetry" {
		t.Fatalf("expected telemetry event, got %+v", appendedEvent)
	}
	if obsStore.created == nil && obsStore.updated == nil {
		t.Fatalf("expected persisted observation, got created=%+v updated=%+v", obsStore.created, obsStore.updated)
	}
	wantTelemetryAt := mustParseTime(t, "2026-01-01T00:06:00Z")
	persisted := obsStore.updated
	if persisted == nil {
		persisted = obsStore.created
	}
	if persisted.LatestTelemetryAt == nil || !persisted.LatestTelemetryAt.Equal(wantTelemetryAt) {
		t.Fatalf("expected latest_telemetry_at %v, got %v", wantTelemetryAt, persisted.LatestTelemetryAt)
	}
	if obs.TargetEntityID == nil || *obs.TargetEntityID != targetEntityID {
		t.Fatalf("expected target_entity_id %q, got %v", targetEntityID, obs.TargetEntityID)
	}
	if !bytes.Contains(obs.JSON, []byte(`"history_object_id"`)) {
		t.Fatalf("expected current observation JSON to include history_object_id, got %s", string(obs.JSON))
	}
}

type ingestObservationStore struct {
	captureObservationStore
	updateCalls int
	firstUpdate error
}

func (s *ingestObservationStore) UpdateObservation(ctx context.Context, obs *model.Observation) error {
	s.updateCalls++
	if s.updateCalls == 1 && s.firstUpdate != nil {
		return s.firstUpdate
	}
	return s.captureObservationStore.UpdateObservation(ctx, obs)
}

func observationIngestTestFixtures(t *testing.T) (ObservationFunctions, *ingestObservationStore, *fakeObjectGateway) {
	t.Helper()
	obsStore := &ingestObservationStore{}
	var createdObject *model.Object
	objectStore := &fakeObjectStore{
		createFn: func(_ context.Context, obj *model.Object) error {
			cp := *obj
			createdObject = &cp
			return nil
		},
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if createdObject != nil && createdObject.ObjectID == objectID {
				return createdObject, nil
			}
			return nil, model.ErrNotFound
		},
	}
	objectGateway := &fakeObjectGateway{ObjectStore: objectStore}
	entityStore := &fakeEntityStore{
		getFn: func(_ context.Context, id string) (*model.Entity, error) {
			switch id {
			case "asset_001":
				return &model.Entity{EntityID: id, Type: model.EntityTypeAsset}, nil
			case "track_001":
				return &model.Entity{EntityID: id, Type: model.EntityTypeTrack}, nil
			default:
				return nil, model.ErrNotFound
			}
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator()).
		WithObjectGateway(objectGateway).
		WithEntityStore(entityStore)
	return f, obsStore, objectGateway
}

func TestObservationFunctions_IngestObservationTelemetryReconcilesOnVersionConflict(t *testing.T) {
	f, obsStore, objectGateway := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore.byID = map[string]*model.Observation{
		"obs_001": {
			ObservationID: "obs_001",
			SourceAssetID: "asset_001",
			StartedAt:     startedAt,
			Version:       3,
			JSON:          minimumObservationJSON,
		},
	}
	obsStore.firstUpdate = model.ErrVersionConflict

	targetEntityID := "track_001"
	obs, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID:  "obs_001",
		SourceAssetID:  "asset_001",
		TargetEntityID: &targetEntityID,
		TelemetryJSON:  []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
	})
	if err != nil {
		t.Fatalf("IngestObservationTelemetry failed: %v", err)
	}
	if obsStore.updateCalls < 2 {
		t.Fatalf("expected reconcile to retry update, got %d update calls", obsStore.updateCalls)
	}
	if len(objectGateway.appended) != 1 {
		t.Fatalf("expected one history append, got %d", len(objectGateway.appended))
	}
	wantTelemetryAt := mustParseTime(t, "2026-01-01T00:06:00Z")
	if obs.LatestTelemetryAt == nil || !obs.LatestTelemetryAt.Equal(wantTelemetryAt) {
		t.Fatalf("expected latest_telemetry_at %v, got %v", wantTelemetryAt, obs.LatestTelemetryAt)
	}
	if !bytes.Contains(obs.JSON, []byte(`"history_object_id"`)) {
		t.Fatalf("expected observation JSON to include history_object_id after reconcile, got %s", string(obs.JSON))
	}
}

func TestObservationFunctions_IngestObservationTelemetryReturnsImmediatelyOnNonVersionUpdateError(t *testing.T) {
	f, obsStore, objectGateway := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore.byID = map[string]*model.Observation{
		"obs_001": {
			ObservationID: "obs_001",
			SourceAssetID: "asset_001",
			StartedAt:     startedAt,
			Version:       3,
			JSON:          minimumObservationJSON,
		},
	}
	obsStore.firstUpdate = model.ErrDatabaseError

	targetEntityID := "track_001"
	_, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID:  "obs_001",
		SourceAssetID:  "asset_001",
		TargetEntityID: &targetEntityID,
		TelemetryJSON:  []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
	})
	if !errors.Is(err, model.ErrDatabaseError) {
		t.Fatalf("expected ErrDatabaseError, got %v", err)
	}
	if obsStore.updateCalls != 1 {
		t.Fatalf("expected single update attempt, got %d", obsStore.updateCalls)
	}
	if obsStore.updated != nil {
		t.Fatalf("expected observation update to fail before persist, got %+v", obsStore.updated)
	}
	if len(objectGateway.appended) != 1 {
		t.Fatalf("expected history append before failed update, got %d appends", len(objectGateway.appended))
	}
}

func TestObservationFunctions_IngestObservationTelemetryRejectsMismatchedExistingHistoryObject(t *testing.T) {
	obsStore := &captureObservationStore{}
	historyObjectID := ObservationHistoryObjectID("obs_001")
	objectGateway := &fakeObjectGateway{ObjectStore: &fakeObjectStore{
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if objectID != historyObjectID {
				return nil, model.ErrNotFound
			}
			return &model.Object{
				ObjectID:  objectID,
				Type:      model.ObjectTypeLog,
				OwnerType: model.OwnerTypeObservation,
				OwnerID:   "obs_other",
			}, nil
		},
	}}
	entityStore := &fakeEntityStore{
		getFn: func(_ context.Context, id string) (*model.Entity, error) {
			switch id {
			case "asset_001":
				return &model.Entity{EntityID: id, Type: model.EntityTypeAsset}, nil
			case "track_001":
				return &model.Entity{EntityID: id, Type: model.EntityTypeTrack}, nil
			default:
				return nil, model.ErrNotFound
			}
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator()).
		WithObjectGateway(objectGateway).
		WithEntityStore(entityStore)

	targetEntityID := "track_001"
	_, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID:  "obs_001",
		SourceAssetID:  "asset_001",
		TargetEntityID: &targetEntityID,
		TelemetryJSON:  []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
		StartedAt:      mustParseTime(t, "2026-01-01T00:00:00Z"),
	})
	if !errors.Is(err, model.ErrConflict) {
		t.Fatalf("expected ErrConflict for mismatched history object, got %v", err)
	}
	if len(objectGateway.appended) != 0 {
		t.Fatalf("expected no history append on conflict, got %d", len(objectGateway.appended))
	}
	if obsStore.created != nil || obsStore.updated != nil {
		t.Fatalf("expected observation write to be skipped on conflict, got created=%+v updated=%+v", obsStore.created, obsStore.updated)
	}
}

func TestGenerateEventIDIsDeterministicForEquivalentTelemetry(t *testing.T) {
	telemetryA := []byte(`{"kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`)
	telemetryB := []byte(`{"data":{"longitude":-74.0,"latitude":40.7},"kind":"point"}`)
	ts := "2026-01-01T00:06:00Z"
	idA := generateEventID("obs_001", observationEventTelemetry, ts, telemetryA)
	idB := generateEventID("obs_001", observationEventTelemetry, ts, telemetryB)
	if idA != idB {
		t.Fatalf("expected deterministic event IDs, got %q and %q", idA, idB)
	}
}

func TestGenerateEventIDIsDeterministicForEquivalentIdentityPatch(t *testing.T) {
	payloadA := []byte(`{"previous":null,"current":{"callsign":"ALPHA"}}`)
	payloadB := []byte(`{"current":{"callsign":"ALPHA"},"previous":null}`)
	ts := "2026-01-01T00:00:00Z"
	idA := generateEventID("obs_001", observationEventIdentityPatch, ts, payloadA)
	idB := generateEventID("obs_001", observationEventIdentityPatch, ts, payloadB)
	if idA != idB {
		t.Fatalf("expected deterministic event IDs, got %q and %q", idA, idB)
	}
}

func TestObservationFunctions_AppendIdentityPatchDedupesHistoryEvent(t *testing.T) {
	obsStore := &captureObservationStore{}
	var createdObject *model.Object
	objectStore := &fakeObjectStore{
		createFn: func(_ context.Context, obj *model.Object) error {
			cp := *obj
			createdObject = &cp
			return nil
		},
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if createdObject != nil && createdObject.ObjectID == objectID {
				return createdObject, nil
			}
			return nil, model.ErrNotFound
		},
	}
	objectGateway := &fakeObjectGateway{ObjectStore: objectStore}
	f := NewObservationFunctions(obsStore, testLogger(), fakeProtocolValidator{}).
		WithObjectGateway(objectGateway)

	effectiveAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     effectiveAt,
		Version:       1,
		JSON:          minimumObservationJSON,
	}
	current := json.RawMessage(`{"callsign":"ALPHA"}`)

	if err := f.appendIdentityPatchIfNeeded(context.Background(), obs, nil, current, effectiveAt); err != nil {
		t.Fatalf("first appendIdentityPatchIfNeeded failed: %v", err)
	}
	if err := f.appendIdentityPatchIfNeeded(context.Background(), obs, nil, current, effectiveAt); err != nil {
		t.Fatalf("second appendIdentityPatchIfNeeded failed: %v", err)
	}
	if len(objectGateway.appended) != 1 {
		t.Fatalf("expected one history append on retry, got %d", len(objectGateway.appended))
	}
	var appendedEvent map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(objectGateway.appended[0].data), &appendedEvent); err != nil {
		t.Fatalf("failed to unmarshal appended history event: %v", err)
	}
	if appendedEvent["event_type"] != observationEventIdentityPatch {
		t.Fatalf("expected identity_patch event, got %+v", appendedEvent)
	}
	if !bytes.Contains(obs.JSON, []byte(`"callsign":"ALPHA"`)) {
		t.Fatalf("expected observation identity to be applied, got %s", string(obs.JSON))
	}
}

func TestTaskFunctions_ValidateRequiredFields(t *testing.T) {
	f := TaskFunctions{}
	task := &model.Task{TaskID: "task_001", AssetID: "asset_001", CommandCatalogObjectID: "cmd_001", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateTask(nil, task); err == nil {
		t.Fatal("expected error for empty status")
	}
	task.Status = model.TaskStatusPending
	task.AssetID = ""
	if err := f.CreateTask(nil, task); err == nil {
		t.Fatal("expected error for empty asset_id")
	}
	task.AssetID = "asset_001"
	task.CommandCatalogObjectID = ""
	if err := f.CreateTask(nil, task); err == nil {
		t.Fatal("expected error for empty command_catalog_object_id")
	}
}

func TestTaskFunctions_RejectsNonCommandCatalogObject(t *testing.T) {
	f := NewTaskFunctions(fakeTaskStore{}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog}, nil
	}}, &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`)}, nil
	}}, fakeIdempotencyStore{}, testLogger(), testProtoValidator())
	task := &model.Task{TaskID: "task_001", Status: model.TaskStatusPending, AssetID: "asset_001", CommandCatalogObjectID: "obj_001", JSON: []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{}}}`)}
	if err := f.CreateTask(context.Background(), task); err == nil {
		t.Fatal("expected task validation failure")
	}
}

func TestObjectFunctions_ValidateRequiredFields(t *testing.T) {
	f := ObjectFunctions{}
	obj := &model.Object{ObjectID: "obj_001", OwnerType: model.OwnerTypeSystem, OwnerID: "system", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObject(nil, obj); err == nil {
		t.Fatal("expected error for empty type")
	}
	obj.Type = model.ObjectTypeLog
	obj.OwnerType = ""
	if err := f.CreateObject(nil, obj); err == nil {
		t.Fatal("expected error for empty owner_type")
	}
	obj.OwnerType = model.OwnerTypeSystem
	obj.OwnerID = ""
	if err := f.CreateObject(nil, obj); err == nil {
		t.Fatal("expected error for empty owner_id")
	}
}

func TestFunctions_RejectNilModels(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		err  error
	}{
		{name: "entity CreateEntity", err: EntityFunctions{}.CreateEntity(ctx, nil)},
		{name: "entity UpdateEntity", err: EntityFunctions{}.UpdateEntity(ctx, nil)},
		{name: "entity UpsertEntity", err: EntityFunctions{}.UpsertEntity(ctx, nil)},
		{name: "object CreateObject", err: ObjectFunctions{}.CreateObject(ctx, nil)},
		{name: "object UpdateObject", err: ObjectFunctions{}.UpdateObject(ctx, nil)},
		{name: "object UpsertObject", err: ObjectFunctions{}.UpsertObject(ctx, nil)},
		{name: "object UpdateObjectManifest", err: ObjectFunctions{}.UpdateObjectManifest(ctx, "obj_001", nil)},
		{name: "task CreateTask", err: TaskFunctions{}.CreateTask(ctx, nil)},
		{name: "task UpdateTask", err: TaskFunctions{}.UpdateTask(ctx, nil)},
		{name: "task UpsertTask", err: TaskFunctions{}.UpsertTask(ctx, nil)},
		{name: "observation CreateObservation", err: ObservationFunctions{}.CreateObservation(ctx, nil)},
		{name: "observation UpdateObservation", err: ObservationFunctions{}.UpdateObservation(ctx, nil)},
		{name: "observation UpsertObservation", err: ObservationFunctions{}.UpsertObservation(ctx, nil)},
	}
	for _, tt := range tests {
		if tt.err == nil {
			t.Fatalf("expected error for nil %s model", tt.name)
		}
	}
}

func TestObjectFunctions_RejectUnsafeObjectID(t *testing.T) {
	f := ObjectFunctions{}
	obj := &model.Object{ObjectID: "../obj", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObject(context.Background(), obj); err == nil {
		t.Fatal("expected error for unsafe object_id")
	}
}

func TestObjectFunctions_UpdateObjectManifestRejectsNil(t *testing.T) {
	f := ObjectFunctions{}
	if err := f.UpdateObjectManifest(context.Background(), "obj_001", nil); err == nil {
		t.Fatal("expected error for nil manifest")
	}
}

func TestObjectFunctions_CreateObjectRecoversPendingIdempotencyClaim(t *testing.T) {
	created := false
	completed := false
	pg := &fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			if created {
				return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}, nil
			}
			return nil, model.ErrNotFound
		},
		createFn: func(context.Context, *model.Object) error {
			created = true
			return nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return nil, model.ErrNotFound
		},
	}
	idem := fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_001", Status: store.IdempotencyStatusPending}, false, nil
		},
		markCompletedFn: func(context.Context, string, string) error {
			completed = true
			return nil
		},
	}
	f := newTestObjectFunctions(pg, idem, testLogger(), testProtoValidator())

	if err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("client-1")); err != nil {
		t.Fatalf("expected pending claim recovery to succeed, got %v", err)
	}
	if !created {
		t.Fatal("expected object creation to resume for pending claim")
	}
	if !completed {
		t.Fatal("expected idempotency key to be marked completed")
	}
}

func TestObjectFunctions_CreateObjectRejectsIdempotencyKeyBoundToAnotherObject(t *testing.T) {
	f := newTestObjectFunctions(&fakeObjectStore{}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_other", Status: store.IdempotencyStatusCompleted}, false, nil
		},
	}, testLogger(), testProtoValidator())

	err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("client-1"))
	if !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}
}

func TestObjectFunctions_CreateObjectWithFreshIdempotencyKeyStillConflictsOnDuplicateID(t *testing.T) {
	markedFailed := false
	pg := &fakeObjectStore{
		createFn: func(context.Context, *model.Object) error { return model.ErrConflict },
	}
	f := newTestObjectFunctions(pg, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger(), testProtoValidator())

	err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("fresh-key"))
	if !errors.Is(err, model.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if !markedFailed {
		t.Fatal("expected fresh idempotency claim to be marked failed on duplicate object")
	}
}

func TestObjectFunctions_CreateObjectMarksClaimFailedOnValidationError(t *testing.T) {
	markedFailed := false
	f := newTestObjectFunctions(&fakeObjectStore{}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger(), fakeProtocolValidator{
		objectIssues: []protocol.ValidationIssue{{Field: "json", Code: "invalid_json", Message: "invalid"}},
	})

	err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("fresh-key"))
	if err == nil {
		t.Fatal("expected validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if !markedFailed {
		t.Fatal("expected fresh object idempotency claim to be marked failed on validation error")
	}
}

func TestObjectFunctions_CreateObjectJoinsMarkFailedErrorOnValidationFailure(t *testing.T) {
	markErr := errors.New("mark failed")
	f := newTestObjectFunctions(&fakeObjectStore{}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			return markErr
		},
	}, testLogger(), fakeProtocolValidator{
		objectIssues: []protocol.ValidationIssue{{Field: "json", Code: "invalid_json", Message: "invalid"}},
	})

	err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("fresh-key"))
	if err == nil {
		t.Fatal("expected joined error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError in joined error, got %T: %v", err, err)
	}
	if !errors.Is(err, markErr) {
		t.Fatalf("expected joined error to include mark failure, got %v", err)
	}
}

func TestTaskFunctions_CreateTaskRecoversPendingIdempotencyClaim(t *testing.T) {
	created := false
	completed := false
	taskStore := fakeTaskStore{
		createFn: func(context.Context, *model.Task) error {
			created = true
			return nil
		},
		getFn: func(context.Context, string) (*model.Task, error) {
			if created {
				return &model.Task{TaskID: "task_001", Status: model.TaskStatusPending, AssetID: "asset_001", CommandCatalogObjectID: "cmd_001"}, nil
			}
			return nil, model.ErrNotFound
		},
	}
	objectStore := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{}}]}`)}, nil
	}}
	idem := fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "task_001", Status: store.IdempotencyStatusPending}, false, nil
		},
		markCompletedFn: func(context.Context, string, string) error {
			completed = true
			return nil
		},
	}
	f := NewTaskFunctions(taskStore, objectStore, &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`)}, nil
	}}, idem, testLogger(), testProtoValidator())

	if err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{}}}`),
	}, WithIdempotencyKey("client-1")); err != nil {
		t.Fatalf("expected pending task claim recovery to succeed, got %v", err)
	}
	if !created {
		t.Fatal("expected task creation to resume for pending claim")
	}
	if !completed {
		t.Fatal("expected task idempotency key to be marked completed")
	}
}

func TestTaskFunctions_CreateTaskWithFreshIdempotencyKeyStillConflictsOnDuplicateID(t *testing.T) {
	markedFailed := false
	taskStore := fakeTaskStore{
		createFn: func(context.Context, *model.Task) error { return model.ErrConflict },
	}
	objectStore := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{}}]}`)}, nil
	}}
	f := NewTaskFunctions(taskStore, objectStore, &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`)}, nil
	}}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "task_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger(), testProtoValidator())

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{}}}`),
	}, WithIdempotencyKey("fresh-key"))
	if !errors.Is(err, model.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if !markedFailed {
		t.Fatal("expected fresh task idempotency claim to be marked failed on duplicate task")
	}
}

func TestTaskFunctions_CreateTaskMarksClaimFailedOnValidationError(t *testing.T) {
	markedFailed := false
	f := NewTaskFunctions(fakeTaskStore{}, &fakeObjectStore{}, &fakeEntityStore{}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "task_001", Status: store.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger(), fakeProtocolValidator{
		taskIssues: []protocol.ValidationIssue{{Field: "json", Code: "invalid_json", Message: "invalid"}},
	})

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
	}, WithIdempotencyKey("fresh-key"))
	if err == nil {
		t.Fatal("expected validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if !markedFailed {
		t.Fatal("expected fresh task idempotency claim to be marked failed on validation error")
	}
}

func TestObjectFunctions_UpdateObjectManifestPublishesMutation(t *testing.T) {
	publisher := &capturePublisher{}
	pg := &fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system", Version: 7}, nil
		},
	}
	f := newTestObjectFunctions(pg, fakeIdempotencyStore{}, testLogger(), testProtoValidator(), publisher)

	manifest := model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{
		"data.txt": {Size: 4, UpdatedAt: mustParseTime(t, "2026-05-05T00:00:00Z")},
	}})
	if err := f.UpdateObjectManifest(context.Background(), "obj_001", manifest); err != nil {
		t.Fatalf("update manifest failed: %v", err)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one object mutation event, got %d", len(publisher.events))
	}
	event := publisher.events[0]
	if event.GetResource() != "object" || event.GetOperation() != "updated" || event.GetResourceId() != "obj_001" {
		t.Fatalf("unexpected mutation event: %+v", event)
	}
	if event.GetObject().GetVersion() != 7 {
		t.Fatalf("expected object snapshot version 7, got %d", event.GetObject().GetVersion())
	}
}

func TestModelErrors_IsCoreError(t *testing.T) {
	wrapped := fmt.Errorf("wrapped: %w", model.ErrNotFound)
	if !errors.Is(wrapped, model.ErrNotFound) {
		t.Fatal("expected wrapped core error to match with errors.Is")
	}
}
