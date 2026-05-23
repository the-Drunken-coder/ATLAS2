package function

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

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
	publisher := &capturePublisher{}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator(), publisher).
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
	if countAppendedFilename(objectGateway.appended, ObservationHistoryFilename) != 1 {
		t.Fatalf("expected one history append, got %d", countAppendedFilename(objectGateway.appended, ObservationHistoryFilename))
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
	if len(publisher.events) != 1 {
		t.Fatalf("expected one changefeed event, got %d", len(publisher.events))
	}
	if publisher.events[0].GetResource() != "observation" || publisher.events[0].GetOperation() != "created" {
		t.Fatalf("unexpected changefeed event: resource=%q operation=%q", publisher.events[0].GetResource(), publisher.events[0].GetOperation())
	}
}

func TestObservationFunctions_IngestObservationTelemetrySkipsStaleTelemetrySnapshot(t *testing.T) {
	f, obsStore, objectGateway, _ := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	newerAt := mustParseTime(t, "2026-01-01T00:10:00Z")
	existingJSON := []byte(`{"identity":{"kind":"asset"},"latest_telemetry":{"observed_at":"2026-01-01T00:10:00Z","kind":"point","data":{"latitude":41.0,"longitude":-75.0}},"history_object_id":"obj_hist_obs_001"}`)
	obsStore.byID = map[string]*model.Observation{
		"obs_001": {
			ObservationID:     "obs_001",
			SourceAssetID:     "asset_001",
			StartedAt:         startedAt,
			Version:           2,
			LatestTelemetryAt: &newerAt,
			JSON:              append([]byte(nil), existingJSON...),
		},
	}

	_, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		TelemetryJSON: []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
	})
	if err != nil {
		t.Fatalf("IngestObservationTelemetry failed: %v", err)
	}
	if obsStore.updated == nil {
		t.Fatal("expected observation update")
	}
	if obsStore.updated.LatestTelemetryAt == nil || !obsStore.updated.LatestTelemetryAt.Equal(newerAt) {
		t.Fatalf("expected latest_telemetry_at to remain %v, got %v", newerAt, obsStore.updated.LatestTelemetryAt)
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"latitude":41`)) {
		t.Fatalf("expected current telemetry snapshot unchanged, got %s", string(obsStore.updated.JSON))
	}
	if countAppendedFilename(objectGateway.appended, ObservationHistoryFilename) != 1 {
		t.Fatalf("expected history append for stale telemetry, got %d", countAppendedFilename(objectGateway.appended, ObservationHistoryFilename))
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

func observationIngestTestFixtures(t *testing.T) (ObservationFunctions, *ingestObservationStore, *fakeObjectGateway, *capturePublisher) {
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
	publisher := &capturePublisher{}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator(), publisher).
		WithObjectGateway(objectGateway).
		WithEntityStore(entityStore)
	return f, obsStore, objectGateway, publisher
}

func TestObservationFunctions_IngestObservationTelemetryReconcilesOnVersionConflict(t *testing.T) {
	f, obsStore, objectGateway, publisher := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore.byID = map[string]*model.Observation{
		"obs_001": {
			ObservationID: "obs_001",
			SourceAssetID: "asset_001",
			StartedAt:     startedAt,
			Version:       3,
			JSON:          testObservationJSON,
		},
	}
	obsStore.firstUpdate = model.ErrVersionConflict

	endedAt := mustParseTime(t, "2026-01-01T01:00:00Z")
	obs, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		EndedAt:       &endedAt,
		TelemetryJSON:  []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
	})
	if err != nil {
		t.Fatalf("IngestObservationTelemetry failed: %v", err)
	}
	if obs.EndedAt == nil || !obs.EndedAt.Equal(endedAt) {
		t.Fatalf("expected ended_at %v after reconcile, got %v", endedAt, obs.EndedAt)
	}
	if obsStore.updateCalls < 2 {
		t.Fatalf("expected reconcile to retry update, got %d update calls", obsStore.updateCalls)
	}
	if countAppendedFilename(objectGateway.appended, ObservationHistoryFilename) != 1 {
		t.Fatalf("expected one history append, got %d", countAppendedFilename(objectGateway.appended, ObservationHistoryFilename))
	}
	wantTelemetryAt := mustParseTime(t, "2026-01-01T00:06:00Z")
	if obs.LatestTelemetryAt == nil || !obs.LatestTelemetryAt.Equal(wantTelemetryAt) {
		t.Fatalf("expected latest_telemetry_at %v, got %v", wantTelemetryAt, obs.LatestTelemetryAt)
	}
	if !bytes.Contains(obs.JSON, []byte(`"history_object_id"`)) {
		t.Fatalf("expected observation JSON to include history_object_id after reconcile, got %s", string(obs.JSON))
	}
	if len(publisher.events) != 1 || publisher.events[0].GetOperation() != "updated" {
		t.Fatalf("expected one updated changefeed event, got %#v", publisher.events)
	}
}

func TestObservationFunctions_IngestObservationTelemetryReturnsImmediatelyOnNonVersionUpdateError(t *testing.T) {
	f, obsStore, objectGateway, _ := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore.byID = map[string]*model.Observation{
		"obs_001": {
			ObservationID: "obs_001",
			SourceAssetID: "asset_001",
			StartedAt:     startedAt,
			Version:       3,
			JSON:          testObservationJSON,
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
	if len(objectGateway.appended) != 0 {
		t.Fatalf("expected no history append when update fails, got %d appends", len(objectGateway.appended))
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

func TestObservationFunctions_IngestTelemetryDedupesHistoryEvent(t *testing.T) {
	f, obsStore, objectGateway, _ := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore.byID = map[string]*model.Observation{
		"obs_001": {
			ObservationID: "obs_001",
			SourceAssetID: "asset_001",
			StartedAt:     startedAt,
			Version:       1,
			JSON:          testObservationJSON,
		},
	}
	telemetry := []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`)
	ingest := ObservationTelemetryIngest{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		TelemetryJSON: telemetry,
	}
	if _, err := f.IngestObservationTelemetry(context.Background(), ingest); err != nil {
		t.Fatalf("first ingest failed: %v", err)
	}
	if _, err := f.IngestObservationTelemetry(context.Background(), ingest); err != nil {
		t.Fatalf("second ingest failed: %v", err)
	}
	if countAppendedFilename(objectGateway.appended, ObservationHistoryFilename) != 1 {
		t.Fatalf("expected one history append for duplicate telemetry, got %d", countAppendedFilename(objectGateway.appended, ObservationHistoryFilename))
	}
}

func TestObservationFunctions_IngestObservationTelemetryRejectsMismatchedSourceAssetID(t *testing.T) {
	f, obsStore, _, _ := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore.byID = map[string]*model.Observation{
		"obs_001": {
			ObservationID: "obs_001",
			SourceAssetID: "asset_001",
			StartedAt:     startedAt,
			Version:       1,
			JSON:          testObservationJSON,
		},
	}
	_, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID: "obs_001",
		SourceAssetID: "asset_999",
		TelemetryJSON: []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
	})
	if err == nil {
		t.Fatal("expected error for mismatched source_asset_id")
	}
	fieldErr, ok := err.(*model.FieldError)
	if !ok || fieldErr.Field != "source_asset_id" {
		t.Fatalf("expected field error on source_asset_id, got %T: %v", err, err)
	}
}

func TestObservationFunctions_IngestObservationTelemetryRejectsMismatchedTargetEntityID(t *testing.T) {
	f, obsStore, _, _ := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	targetEntityID := "track_001"
	obsStore.byID = map[string]*model.Observation{
		"obs_001": {
			ObservationID:  "obs_001",
			SourceAssetID:  "asset_001",
			TargetEntityID: &targetEntityID,
			StartedAt:      startedAt,
			Version:        1,
			JSON:           testObservationJSON,
		},
	}
	wrongTarget := "track_999"
	_, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID:  "obs_001",
		SourceAssetID:  "asset_001",
		TargetEntityID: &wrongTarget,
		TelemetryJSON:  []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
	})
	if err == nil {
		t.Fatal("expected error for mismatched target_entity_id")
	}
	fieldErr, ok := err.(*model.FieldError)
	if !ok || fieldErr.Field != "target_entity_id" {
		t.Fatalf("expected field error on target_entity_id, got %T: %v", err, err)
	}
}

func TestObservationFunctions_IngestObservationTelemetryRejectsEmptySourceAssetID(t *testing.T) {
	f, obsStore, _, _ := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore.byID = map[string]*model.Observation{
		"obs_001": {
			ObservationID: "obs_001",
			SourceAssetID: "asset_001",
			StartedAt:     startedAt,
			Version:       1,
			JSON:          testObservationJSON,
		},
	}
	_, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID: "obs_001",
		SourceAssetID: "",
		TelemetryJSON: []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
	})
	if err == nil {
		t.Fatal("expected error for empty source_asset_id")
	}
	fieldErr, ok := err.(*model.FieldError)
	if !ok || fieldErr.Field != "source_asset_id" {
		t.Fatalf("expected field error on source_asset_id, got %T: %v", err, err)
	}
}
