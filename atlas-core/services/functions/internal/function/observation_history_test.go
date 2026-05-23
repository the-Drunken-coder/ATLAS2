package function

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

// datastorageStyleObjectGateway returns model.ErrNotFound for missing object files,
// matching datastorage ReadObjectFile behavior.
type datastorageStyleObjectGateway struct {
	fakeObjectGateway
}

func (g *datastorageStyleObjectGateway) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	if _, err := g.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	if g.files != nil {
		if fileData, ok := g.files[objectID][filename]; ok {
			return append([]byte(nil), fileData...), nil
		}
	}
	return nil, model.ErrNotFound
}

func TestHistoryContainsEventID_MissingHistoryFileIsEmpty(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	objectStore := &fakeObjectStore{
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if objectID != historyObjectID {
				return nil, model.ErrNotFound
			}
			return &model.Object{ObjectID: objectID}, nil
		},
	}
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).
		WithObjectGateway(&datastorageStyleObjectGateway{fakeObjectGateway: fakeObjectGateway{ObjectStore: objectStore}})

	exists, err := f.historyContainsEventID(context.Background(), historyObjectID, "obs_evt_missing")
	if err != nil {
		t.Fatalf("expected no error for missing history file, got %v", err)
	}
	if exists {
		t.Fatal("expected event to be absent when history file is missing")
	}
}

func TestHistoryContainsEventID_FindsEventInExistingHistory(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	line, err := buildTelemetryHistoryLine(
		"obs_001",
		1,
		telemetryEnvelope{
			ObservedAt: "2026-01-01T00:06:00Z",
			Kind:       "point",
			Data:       json.RawMessage(`{"latitude":40.7,"longitude":-74.0}`),
		},
		time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("buildTelemetryHistoryLine: %v", err)
	}
	eventID, err := historyEventIDFromLine(line)
	if err != nil {
		t.Fatalf("historyEventIDFromLine: %v", err)
	}

	gateway := &datastorageStyleObjectGateway{fakeObjectGateway: fakeObjectGateway{
		ObjectStore: &fakeObjectStore{
			getFn: func(_ context.Context, objectID string) (*model.Object, error) {
				return &model.Object{
					ObjectID:  objectID,
					Type:      model.ObjectTypeObservationHistory,
					OwnerType: model.OwnerTypeObservation,
					OwnerID:   "obs_001",
					JSON:      []byte(`{"format_version":"v1"}`),
				}, nil
			},
			updateFn: func(_ context.Context, obj *model.Object) error { return nil },
		},
		files: map[string]map[string][]byte{
			historyObjectID: {ObservationHistoryFilename: line},
		},
	}}
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).WithObjectGateway(gateway)

	exists, err := f.historyContainsEventID(context.Background(), historyObjectID, eventID)
	if err != nil {
		t.Fatalf("historyContainsEventID failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected event %q to exist in history", eventID)
	}
}

func TestHistoryContainsEventID_FallsBackToHistoryWhenSidecarMisses(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	line, err := buildTelemetryHistoryLine(
		"obs_001",
		1,
		telemetryEnvelope{
			ObservedAt: "2026-01-01T00:06:00Z",
			Kind:       "point",
			Data:       json.RawMessage(`{"latitude":40.7,"longitude":-74.0}`),
		},
		time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("buildTelemetryHistoryLine: %v", err)
	}
	eventID, err := historyEventIDFromLine(line)
	if err != nil {
		t.Fatalf("historyEventIDFromLine: %v", err)
	}
	otherID := "obs_evt_other_sidecar_only"
	sidecar := otherID + "\n"

	gateway := &datastorageStyleObjectGateway{fakeObjectGateway: fakeObjectGateway{
		ObjectStore: &fakeObjectStore{
			getFn: func(_ context.Context, objectID string) (*model.Object, error) {
				return &model.Object{
					ObjectID:  objectID,
					Type:      model.ObjectTypeObservationHistory,
					OwnerType: model.OwnerTypeObservation,
					OwnerID:   "obs_001",
				}, nil
			},
		},
		files: map[string]map[string][]byte{
			historyObjectID: {
				ObservationHistoryFilename:         line,
				ObservationHistoryEventIDsFilename: []byte(sidecar),
			},
		},
	}}
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).WithObjectGateway(gateway)

	exists, err := f.historyContainsEventID(context.Background(), historyObjectID, eventID)
	if err != nil {
		t.Fatalf("historyContainsEventID failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected event %q from history.ndjson when sidecar missed it", eventID)
	}
}

func TestAppendHistoryEvent_IndexUpdateFailureStillSucceeds(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	line, err := buildTelemetryHistoryLine(
		"obs_001",
		1,
		telemetryEnvelope{
			ObservedAt: "2026-01-01T00:06:00Z",
			Kind:       "point",
			Data:       json.RawMessage(`{"latitude":40.7,"longitude":-74.0}`),
		},
		time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("buildTelemetryHistoryLine: %v", err)
	}

	updateCalls := 0
	storedObject := &model.Object{
		ObjectID:  historyObjectID,
		Type:      model.ObjectTypeObservationHistory,
		OwnerType: model.OwnerTypeObservation,
		OwnerID:   "obs_001",
		JSON:      []byte(`{"format_version":"v1","extra":{"event_id_index":["obs_evt_old"],"event_id_index_complete":true}}`),
	}
	gateway := &datastorageStyleObjectGateway{fakeObjectGateway: fakeObjectGateway{
		ObjectStore: &fakeObjectStore{
			getFn: func(_ context.Context, objectID string) (*model.Object, error) {
				cp := *storedObject
				return &cp, nil
			},
			updateFn: func(_ context.Context, obj *model.Object) error {
				updateCalls++
				if updateCalls == 1 {
					return model.ErrConflict
				}
				storedObject.JSON = append([]byte(nil), obj.JSON...)
				return nil
			},
		},
	}}
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).WithObjectGateway(gateway)

	if err := f.appendHistoryEvent(context.Background(), historyObjectID, line); err != nil {
		t.Fatalf("appendHistoryEvent: %v", err)
	}
	if len(gateway.appended) != 2 {
		t.Fatalf("expected history and event_ids appends, got %d", len(gateway.appended))
	}
	if gateway.files[historyObjectID][ObservationHistoryEventIDsFilename] == nil {
		t.Fatal("expected event_ids.ndjson sidecar after append")
	}
	if !bytes.Equal(bytes.TrimSpace(gateway.files[historyObjectID][ObservationHistoryFilename]), bytes.TrimSpace(line)) {
		t.Fatal("expected history file to contain appended event line")
	}
	state := parseHistoryEventIDIndex(storedObject.JSON)
	if state.complete {
		t.Fatal("expected event_id_index_complete false after index update failure")
	}
}

func TestAppendHistoryEventIfAbsent_ConcurrentDedup(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	line, err := buildTelemetryHistoryLine(
		"obs_001",
		1,
		telemetryEnvelope{
			ObservedAt: "2026-01-01T00:06:00Z",
			Kind:       "point",
			Data:       json.RawMessage(`{"latitude":40.7,"longitude":-74.0}`),
		},
		time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("buildTelemetryHistoryLine: %v", err)
	}

	gateway := &datastorageStyleObjectGateway{fakeObjectGateway: fakeObjectGateway{
		ObjectStore: &fakeObjectStore{
			getFn: func(_ context.Context, objectID string) (*model.Object, error) {
				return &model.Object{
					ObjectID:  objectID,
					Type:      model.ObjectTypeObservationHistory,
					OwnerType: model.OwnerTypeObservation,
					OwnerID:   "obs_001",
					JSON:      []byte(`{"format_version":"v1"}`),
				}, nil
			},
			updateFn: func(_ context.Context, obj *model.Object) error { return nil },
		},
	}}
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).WithObjectGateway(gateway)

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- f.appendHistoryEventIfAbsent(context.Background(), historyObjectID, line)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("appendHistoryEventIfAbsent: %v", err)
		}
	}

	historyData := gateway.files[historyObjectID][ObservationHistoryFilename]
	lineCount := 0
	for _, part := range bytes.Split(bytes.TrimSpace(historyData), []byte("\n")) {
		if len(bytes.TrimSpace(part)) > 0 {
			lineCount++
		}
	}
	if lineCount != 1 {
		t.Fatalf("expected 1 history line after concurrent dedup, got %d", lineCount)
	}
}

func TestObservationFunctions_IngestObservationTelemetryWithMissingHistoryFile(t *testing.T) {
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
	objectGateway := &datastorageStyleObjectGateway{fakeObjectGateway: fakeObjectGateway{ObjectStore: objectStore}}
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
	f := NewObservationFunctions(obsStore, testLogger(), fakeProtocolValidator{}).
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
	if err != nil {
		t.Fatalf("IngestObservationTelemetry failed: %v", err)
	}
	if countAppendedFilename(objectGateway.appended, ObservationHistoryFilename) != 1 {
		t.Fatalf("expected one history append on first ingest, got %d", countAppendedFilename(objectGateway.appended, ObservationHistoryFilename))
	}
	appendCall := objectGateway.appended[0]
	if appendCall.filename != ObservationHistoryFilename {
		t.Fatalf("unexpected append filename: %q", appendCall.filename)
	}
	var appendedEvent map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(appendCall.data), &appendedEvent); err != nil {
		t.Fatalf("failed to unmarshal appended history event: %v", err)
	}
	if appendedEvent["event_type"] != observationEventTelemetry {
		t.Fatalf("expected telemetry event, got %+v", appendedEvent)
	}
	if obsStore.created == nil && obsStore.updated == nil {
		t.Fatal("expected persisted observation")
	}
}

func TestHistoryContainsEventID_CompleteIndexSkipsFileRead(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	eventID := "obs_evt_indexed"
	indexJSON, err := mergeHistoryEventIDIndexIntoJSON(
		[]byte(`{"format_version":"v1"}`),
		historyEventIDIndexState{
			ids:      map[string]struct{}{eventID: {}},
			complete: true,
		},
	)
	if err != nil {
		t.Fatalf("mergeHistoryEventIDIndexIntoJSON: %v", err)
	}

	gateway := &readCountingGateway{datastorageStyleObjectGateway: datastorageStyleObjectGateway{
		fakeObjectGateway: fakeObjectGateway{
			ObjectStore: &fakeObjectStore{
				getFn: func(_ context.Context, objectID string) (*model.Object, error) {
					return &model.Object{
						ObjectID:  objectID,
						Type:      model.ObjectTypeObservationHistory,
						OwnerType: model.OwnerTypeObservation,
						OwnerID:   "obs_001",
						JSON:      indexJSON,
					}, nil
				},
			},
			files: map[string]map[string][]byte{
				historyObjectID: {ObservationHistoryFilename: []byte("should not be read\n")},
			},
		},
	}}
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).WithObjectGateway(gateway)

	exists, err := f.historyContainsEventID(context.Background(), historyObjectID, eventID)
	if err != nil {
		t.Fatalf("historyContainsEventID failed: %v", err)
	}
	if !exists {
		t.Fatal("expected indexed event to exist")
	}
	if gateway.readFileCalls != 0 {
		t.Fatalf("expected complete index to skip file read, got %d reads", gateway.readFileCalls)
	}
}

func TestHistoryContainsEventID_BootstrapsIndexForLaterChecks(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	line, err := buildTelemetryHistoryLine(
		"obs_001",
		1,
		telemetryEnvelope{
			ObservedAt: "2026-01-01T00:06:00Z",
			Kind:       "point",
			Data:       json.RawMessage(`{"latitude":40.7,"longitude":-74.0}`),
		},
		time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("buildTelemetryHistoryLine: %v", err)
	}
	eventID, err := historyEventIDFromLine(line)
	if err != nil {
		t.Fatalf("historyEventIDFromLine: %v", err)
	}

	var storedObject *model.Object
	historyObject := &model.Object{
		ObjectID:  historyObjectID,
		Type:      model.ObjectTypeObservationHistory,
		OwnerType: model.OwnerTypeObservation,
		OwnerID:   "obs_001",
		JSON:      []byte(`{"format_version":"v1"}`),
	}
	gateway := &readCountingGateway{datastorageStyleObjectGateway: datastorageStyleObjectGateway{
		fakeObjectGateway: fakeObjectGateway{
			ObjectStore: &fakeObjectStore{
				getFn: func(_ context.Context, objectID string) (*model.Object, error) {
					if storedObject != nil && storedObject.ObjectID == objectID {
						cp := *storedObject
						return &cp, nil
					}
					return historyObject, nil
				},
				updateFn: func(_ context.Context, obj *model.Object) error {
					cp := *obj
					storedObject = &cp
					return nil
				},
			},
			files: map[string]map[string][]byte{
				historyObjectID: {ObservationHistoryFilename: line},
			},
		},
	}}
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).WithObjectGateway(gateway)

	exists, err := f.historyContainsEventID(context.Background(), historyObjectID, eventID)
	if err != nil {
		t.Fatalf("first historyContainsEventID failed: %v", err)
	}
	if !exists {
		t.Fatal("expected event in history file")
	}
	if gateway.readFileCalls != 2 {
		t.Fatalf("expected sidecar and history reads during lookup, got %d", gateway.readFileCalls)
	}
	if err := f.bootstrapHistoryEventIDIndex(context.Background(), historyObjectID); err != nil {
		t.Fatalf("bootstrapHistoryEventIDIndex: %v", err)
	}
	if storedObject == nil {
		t.Fatal("expected bootstrap to persist index on history object")
	}
	index := parseHistoryEventIDIndex(storedObject.JSON)
	if !index.complete {
		t.Fatal("expected bootstrapped index to be marked complete")
	}
	if _, ok := index.ids[eventID]; !ok {
		t.Fatalf("expected bootstrapped index to contain %q", eventID)
	}

	gateway.readFileCalls = 0
	exists, err = f.historyContainsEventID(context.Background(), historyObjectID, eventID)
	if err != nil {
		t.Fatalf("second historyContainsEventID failed: %v", err)
	}
	if !exists {
		t.Fatal("expected indexed event on second check")
	}
	if gateway.readFileCalls != 0 {
		t.Fatalf("expected second check to reuse index without file reads, got %d reads", gateway.readFileCalls)
	}
}

func TestHistoryContainsEventID_PropagatesNonNotFoundReadErrors(t *testing.T) {
	wantErr := errors.New("read failed")
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).
		WithObjectGateway(&readErrorGateway{
			fakeObjectGateway: fakeObjectGateway{
				ObjectStore: &fakeObjectStore{
					getFn: func(_ context.Context, objectID string) (*model.Object, error) {
						return &model.Object{ObjectID: objectID, JSON: []byte(`{"format_version":"v1"}`)}, nil
					},
				},
			},
			err: wantErr,
		})

	_, err := f.historyContainsEventID(context.Background(), "obj_hist", "obs_evt_x")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected read error to propagate, got %v", err)
	}
}

type readErrorGateway struct {
	fakeObjectGateway
	err error
}

func (g *readErrorGateway) ReadFile(context.Context, string, string) ([]byte, error) {
	return nil, g.err
}

type readCountingGateway struct {
	datastorageStyleObjectGateway
	readFileCalls int
}

func (g *readCountingGateway) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	g.readFileCalls++
	return g.datastorageStyleObjectGateway.ReadFile(ctx, objectID, filename)
}

func TestReconcileAfterHistoryAppendTelemetrySetsHistoryObjectID(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	stored := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		Version:       1,
		StartedAt:     startedAt,
		JSON:          testObservationJSON,
		CreatedAt:     startedAt,
		UpdatedAt:     startedAt,
	}
	obsStore := &captureObservationStore{byID: map[string]*model.Observation{"obs_001": stored}}

	telemetry := telemetryEnvelope{
		ObservedAt: "2026-01-01T00:06:00Z",
		Kind:       "point",
		Data:       json.RawMessage(`{"latitude":40.7,"longitude":-74.0}`),
	}
	recordedAt := mustParseTime(t, "2026-01-01T00:07:00Z")
	eventLine, err := buildTelemetryHistoryLine("obs_001", 1, telemetry, recordedAt)
	if err != nil {
		t.Fatalf("buildTelemetryHistoryLine: %v", err)
	}

	objectStore := &fakeObjectStore{
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			if objectID != historyObjectID {
				return nil, model.ErrNotFound
			}
			return &model.Object{
				ObjectID:  objectID,
				Type:      model.ObjectTypeObservationHistory,
				OwnerType: model.OwnerTypeObservation,
				OwnerID:   "obs_001",
			}, nil
		},
	}
	objectGateway := &fakeObjectGateway{
		ObjectStore: objectStore,
		files: map[string]map[string][]byte{
			historyObjectID: {ObservationHistoryFilename: eventLine},
		},
	}

	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator()).
		WithObjectGateway(objectGateway)

	if _, err := f.reconcileAfterHistoryAppend(context.Background(), stored, historyObjectID, eventLine, false); err != nil {
		t.Fatalf("reconcileAfterHistoryAppend: %v", err)
	}
	if obsStore.updated == nil {
		t.Fatal("expected observation update from reconcile")
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"history_object_id"`)) {
		t.Fatalf("expected history_object_id in observation JSON after reconcile, got %s", obsStore.updated.JSON)
	}
	var root map[string]any
	if err := json.Unmarshal(obsStore.updated.JSON, &root); err != nil {
		t.Fatalf("unmarshal observation JSON: %v", err)
	}
	if root["history_object_id"] != historyObjectID {
		t.Fatalf("expected history_object_id %q, got %v", historyObjectID, root["history_object_id"])
	}
	wantTelemetryAt := mustParseTime(t, "2026-01-01T00:06:00Z")
	if obsStore.updated.LatestTelemetryAt == nil || !obsStore.updated.LatestTelemetryAt.Equal(wantTelemetryAt) {
		t.Fatalf("expected latest_telemetry_at %v, got %v", wantTelemetryAt, obsStore.updated.LatestTelemetryAt)
	}
}

func TestParseIdentityBytes_AcceptsWhitespacePrefixedObject(t *testing.T) {
	identity, err := parseIdentityBytes([]byte("  {\"callsign\":\"ALPHA\"}  "))
	if err != nil {
		t.Fatalf("parseIdentityBytes: %v", err)
	}
	if string(identity) != `{"callsign":"ALPHA"}` {
		t.Fatalf("expected trimmed canonical object, got %s", identity)
	}
}

func TestParseIdentityBytes_RejectsMalformedJSON(t *testing.T) {
	_, err := parseIdentityBytes([]byte(`{`))
	if err == nil {
		t.Fatal("expected error for malformed identity JSON")
	}
}

func TestCanonicalizeTelemetryJSON_AcceptsWhitespacePrefixedData(t *testing.T) {
	telemetry, err := canonicalizeTelemetryJSON([]byte(`{
		"kind":"point",
		"observed_at":"2026-01-01T00:00:00Z",
		"data": {"latitude":40.7}
	}`))
	if err != nil {
		t.Fatalf("canonicalizeTelemetryJSON: %v", err)
	}
	if string(telemetry.Data) != `{"latitude":40.7}` {
		t.Fatalf("expected trimmed data object, got %s", telemetry.Data)
	}
}

func TestMergeHistoryEventIDIndexIntoJSON_PreservesUnmodeledFields(t *testing.T) {
	original := []byte(`{"format_version":"v1","custom_section":{"note":"keep"},"extra":{"other":"meta"}}`)
	merged, err := mergeHistoryEventIDIndexIntoJSON(original, historyEventIDIndexState{
		ids:      map[string]struct{}{"obs_evt_abc": {}},
		complete: true,
	})
	if err != nil {
		t.Fatalf("mergeHistoryEventIDIndexIntoJSON: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(merged, &root); err != nil {
		t.Fatalf("unmarshal merged json: %v", err)
	}
	if root["custom_section"] == nil {
		t.Fatal("expected custom_section to be preserved")
	}
	extra, ok := root["extra"].(map[string]any)
	if !ok {
		t.Fatalf("expected extra object, got %#v", root["extra"])
	}
	if extra["other"] != "meta" {
		t.Fatalf("expected unrelated extra metadata to be preserved, got %#v", extra["other"])
	}
	index, ok := extra["event_id_index"].([]any)
	if !ok || len(index) != 1 || index[0] != "obs_evt_abc" {
		t.Fatalf("expected event_id_index update, got %#v", extra["event_id_index"])
	}
	if extra["event_id_index_complete"] != true {
		t.Fatalf("expected event_id_index_complete true, got %#v", extra["event_id_index_complete"])
	}
}

func TestPruneHistoryEventIDIndexState_TruncatesAndClearsComplete(t *testing.T) {
	state := historyEventIDIndexState{
		ids:      make(map[string]struct{}),
		complete: true,
	}
	for i := 0; i < maxHistoryEventIDIndexEntries+10; i++ {
		state.ids[fmt.Sprintf("obs_evt_%04d", i)] = struct{}{}
	}
	pruneHistoryEventIDIndexState(&state)
	if len(state.ids) != maxHistoryEventIDIndexEntries {
		t.Fatalf("expected %d ids after prune, got %d", maxHistoryEventIDIndexEntries, len(state.ids))
	}
	if state.complete {
		t.Fatal("expected complete=false after prune")
	}
}

type trackingReadFileGateway struct {
	datastorageStyleObjectGateway
	readFilenames []string
}

func (g *trackingReadFileGateway) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	g.readFilenames = append(g.readFilenames, filename)
	return g.datastorageStyleObjectGateway.ReadFile(ctx, objectID, filename)
}

func TestHistoryContainsEventID_UsesSidecarWithoutReadingHistory(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	eventID := "obs_evt_sidecar"
	gateway := &trackingReadFileGateway{datastorageStyleObjectGateway: datastorageStyleObjectGateway{
		fakeObjectGateway: fakeObjectGateway{
			ObjectStore: &fakeObjectStore{
				getFn: func(_ context.Context, objectID string) (*model.Object, error) {
					return &model.Object{
						ObjectID:  objectID,
						Type:      model.ObjectTypeObservationHistory,
						OwnerType: model.OwnerTypeObservation,
						OwnerID:   "obs_001",
						JSON:      []byte(`{"format_version":"v1"}`),
					}, nil
				},
			},
			files: map[string]map[string][]byte{
				historyObjectID: {
					ObservationHistoryEventIDsFilename: []byte(eventID + "\n"),
				},
			},
		},
	}}
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).WithObjectGateway(gateway)

	exists, err := f.historyContainsEventID(context.Background(), historyObjectID, eventID)
	if err != nil {
		t.Fatalf("historyContainsEventID failed: %v", err)
	}
	if !exists {
		t.Fatal("expected event in sidecar")
	}
	for _, name := range gateway.readFilenames {
		if name == ObservationHistoryFilename {
			t.Fatalf("expected no read of %s, got reads %#v", ObservationHistoryFilename, gateway.readFilenames)
		}
	}
}

func TestRebuildLegacyHistoryIndexes_WritesSidecar(t *testing.T) {
	historyObjectID := ObservationHistoryObjectID("obs_001")
	line, err := buildTelemetryHistoryLine(
		"obs_001",
		1,
		telemetryEnvelope{
			ObservedAt: "2026-01-01T00:06:00Z",
			Kind:       "point",
			Data:       json.RawMessage(`{"latitude":40.7,"longitude":-74.0}`),
		},
		time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("buildTelemetryHistoryLine: %v", err)
	}
	eventID, err := historyEventIDFromLine(line)
	if err != nil {
		t.Fatalf("historyEventIDFromLine: %v", err)
	}

	storedObject := &model.Object{
		ObjectID:  historyObjectID,
		Type:      model.ObjectTypeObservationHistory,
		OwnerType: model.OwnerTypeObservation,
		OwnerID:   "obs_001",
		JSON:      []byte(`{"format_version":"v1"}`),
	}
	gateway := &datastorageStyleObjectGateway{fakeObjectGateway: fakeObjectGateway{
		ObjectStore: &fakeObjectStore{
			getFn: func(_ context.Context, objectID string) (*model.Object, error) {
				cp := *storedObject
				return &cp, nil
			},
			updateFn: func(_ context.Context, obj *model.Object) error {
				storedObject.JSON = append([]byte(nil), obj.JSON...)
				return nil
			},
		},
		files: map[string]map[string][]byte{
			historyObjectID: {ObservationHistoryFilename: line},
		},
	}}
	f := NewObservationFunctions(nil, testLogger(), testProtoValidator()).WithObjectGateway(gateway)

	exists, err := f.historyContainsEventID(context.Background(), historyObjectID, eventID)
	if err != nil {
		t.Fatalf("historyContainsEventID failed: %v", err)
	}
	if !exists {
		t.Fatal("expected event in history file")
	}
	if err := f.bootstrapHistoryEventIDIndex(context.Background(), historyObjectID); err != nil {
		t.Fatalf("bootstrapHistoryEventIDIndex: %v", err)
	}
	sidecar := gateway.files[historyObjectID][ObservationHistoryEventIDsFilename]
	if !bytes.Contains(sidecar, []byte(eventID)) {
		t.Fatalf("expected sidecar to contain %q, got %q", eventID, sidecar)
	}

	tracking := &trackingReadFileGateway{datastorageStyleObjectGateway: datastorageStyleObjectGateway{
		fakeObjectGateway: fakeObjectGateway{
			ObjectStore: &fakeObjectStore{
				getFn: func(_ context.Context, objectID string) (*model.Object, error) {
					cp := *storedObject
					return &cp, nil
				},
			},
			files: gateway.files,
		},
	}}
	f2 := NewObservationFunctions(nil, testLogger(), testProtoValidator()).WithObjectGateway(tracking)
	exists, err = f2.historyContainsEventID(context.Background(), historyObjectID, eventID)
	if err != nil {
		t.Fatalf("second historyContainsEventID failed: %v", err)
	}
	if !exists {
		t.Fatal("expected event on second lookup")
	}
	if len(tracking.readFilenames) != 0 {
		t.Fatalf("expected complete index to avoid file reads, got %#v", tracking.readFilenames)
	}
}
