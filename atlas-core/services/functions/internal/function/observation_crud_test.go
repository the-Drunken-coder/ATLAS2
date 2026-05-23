package function

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

func TestObservationFunctions_ValidateObservationID(t *testing.T) {
	f := ObservationFunctions{}
	obs := &model.Observation{SourceAssetID: "asset_001", JSON: testObservationJSON, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObservation(nil, obs); err == nil {
		t.Fatal("expected error for empty observation_id")
	}
}

func TestObservationFunctions_ValidateSourceAssetID(t *testing.T) {
	f := ObservationFunctions{}
	obs := &model.Observation{ObservationID: "obs_001", JSON: testObservationJSON, CreatedAt: time.Now(), UpdatedAt: time.Now()}
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
	if err := validateObservationJSON(testObservationJSON); err != nil {
		t.Fatalf("expected minimum observation json to be valid, got %v", err)
	}
}

func TestValidateObservationJSON_RejectsNullSectionsAndExtraOnly(t *testing.T) {
	cases := []struct {
		json  []byte
		field string
	}{
		{[]byte(`{"extra":{}}`), "json"},
		{[]byte(`{"extra":{},"identity":null}`), "json.identity"},
		{[]byte(`{"extra":{},"latest_telemetry":null}`), "json.latest_telemetry"},
	}
	for _, tc := range cases {
		err := validateObservationJSON(tc.json)
		if err == nil {
			t.Fatalf("expected error for observation json %q", tc.json)
		}
		fieldErr, ok := err.(*model.FieldError)
		if !ok || fieldErr.Field != tc.field {
			t.Fatalf("expected field error on %s for %q, got %T: %v", tc.field, tc.json, err, err)
		}
	}
	if err := validateObservationJSON([]byte(`{"identity":{"kind":"asset"}}`)); err != nil {
		t.Fatalf("expected valid observation json, got %v", err)
	}
}

func TestValidateObservationJSON_RejectsNullRoot(t *testing.T) {
	for _, jsonBytes := range [][]byte{[]byte(`null`), []byte(` null `)} {
		err := validateObservationJSON(jsonBytes)
		if err == nil {
			t.Fatalf("expected error for observation json %q", jsonBytes)
		}
		fieldErr, ok := err.(*model.FieldError)
		if !ok || fieldErr.Field != "json" {
			t.Fatalf("expected field error on json for %q, got %T: %v", jsonBytes, err, err)
		}
	}
}

func TestObservationFunctions_UpdateObservationPreservesMergedJSONOnIdentitySync(t *testing.T) {
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	existingJSON := []byte(`{"identity":{"kind":"asset"},"extra":{"note":"keep"}}`)
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID: "obs_001",
				SourceAssetID: "asset_001",
				StartedAt:     startedAt,
				Version:       1,
				JSON:          existingJSON,
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
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator()).
		WithObjectGateway(&fakeObjectGateway{ObjectStore: objectStore})

	update := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     startedAt,
		Version:       1,
		JSON:          []byte(`{"identity":{"kind":"vehicle"}}`),
	}
	if err := f.UpdateObservation(context.Background(), update); err != nil {
		t.Fatalf("UpdateObservation failed: %v", err)
	}
	if obsStore.updated == nil {
		t.Fatal("expected follow-up update for identity sync")
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"extra"`)) {
		t.Fatalf("expected merged extra field to persist after identity sync, got %s", obsStore.updated.JSON)
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"kind":"vehicle"`)) {
		t.Fatalf("expected updated identity in JSON, got %s", obsStore.updated.JSON)
	}
}

func TestObservationFunctions_CreateObservationRejectsExtraOnlyJSON(t *testing.T) {
	store := &captureObservationStore{}
	f := NewObservationFunctions(store, testLogger(), fakeProtocolValidator{})
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     time.Now().UTC(),
		JSON:          []byte(`{"extra":{}}`),
	}
	err := f.CreateObservation(context.Background(), obs)
	if err == nil {
		t.Fatal("expected error for extra-only observation json on create")
	}
	fieldErr, ok := err.(*model.FieldError)
	if !ok || fieldErr.Field != "json" {
		t.Fatalf("expected field error on json, got %T: %v", err, err)
	}
}

func TestObservationFunctions_CreateObservationRequiresStartedAt(t *testing.T) {
	store := &captureObservationStore{}
	f := NewObservationFunctions(store, testLogger(), fakeProtocolValidator{})
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		JSON:          testObservationJSON,
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
	if countAppendedFilename(objectGateway.appended, ObservationHistoryFilename) != 1 {
		t.Fatalf("expected one identity_patch history append, got %d", countAppendedFilename(objectGateway.appended, ObservationHistoryFilename))
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
				JSON:          testObservationJSON,
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
	if countAppendedFilename(objectGateway.appended, ObservationHistoryFilename) != 1 {
		t.Fatalf("expected one identity_patch history append, got %d", countAppendedFilename(objectGateway.appended, ObservationHistoryFilename))
	}
}

func TestObservationFunctions_UpdateObservationRejectsIdentityRemovalWithoutTelemetry(t *testing.T) {
	existingStartedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID: "obs_001",
				SourceAssetID: "asset_001",
				StartedAt:     existingStartedAt,
				Version:       1,
				JSON:          []byte(`{"identity":{"kind":"vehicle"}}`),
			},
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator())
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		Version:       1,
		JSON:          []byte(`{"extra":{"note":"cleared"}}`),
	}
	err := f.UpdateObservation(context.Background(), obs)
	if err == nil {
		t.Fatal("expected error when removing identity without latest_telemetry")
	}
	fieldErr, ok := err.(*model.FieldError)
	if !ok || fieldErr.Field != "json.identity" {
		t.Fatalf("expected field error on json.identity, got %T: %v", err, err)
	}
}

func TestObservationFunctions_UpdateObservationRecordsIdentityRemovalWithTelemetry(t *testing.T) {
	existingStartedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID: "obs_001",
				SourceAssetID: "asset_001",
				StartedAt:     existingStartedAt,
				Version:       1,
				JSON:          []byte(`{"identity":{"kind":"vehicle"},"latest_telemetry":{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}}`),
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

	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		Version:       1,
		JSON:          []byte(`{"extra":{"note":"cleared"}}`),
	}
	if err := f.UpdateObservation(context.Background(), obs); err != nil {
		t.Fatalf("UpdateObservation failed: %v", err)
	}
	if obsStore.updated == nil {
		t.Fatal("expected observation update")
	}
	if bytes.Contains(obsStore.updated.JSON, []byte(`"identity"`)) {
		t.Fatalf("expected identity removed from JSON, got %s", string(obsStore.updated.JSON))
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"latest_telemetry"`)) {
		t.Fatalf("expected latest_telemetry preserved, got %s", string(obsStore.updated.JSON))
	}
	if obsStore.updated.LatestIdentityAt == nil {
		t.Fatal("expected latest_identity_at after removal")
	}
	if countAppendedFilename(objectGateway.appended, ObservationHistoryFilename) != 1 {
		t.Fatalf("expected one identity_patch history append, got %d", countAppendedFilename(objectGateway.appended, ObservationHistoryFilename))
	}
}

func TestObservationFunctions_UpdateObservationPreservesStartedAtWhenOmitted(t *testing.T) {
	existingStartedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID: "obs_001",
				SourceAssetID: "asset_001",
				StartedAt:     existingStartedAt,
				Version:       1,
				JSON:          testObservationJSON,
			},
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator())
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		Version:       1,
		JSON:          []byte(`{"identity":{"kind":"asset"},"extra":{"note":"updated"}}`),
	}
	if err := f.UpdateObservation(context.Background(), obs); err != nil {
		t.Fatalf("UpdateObservation failed: %v", err)
	}
	if obsStore.updated == nil {
		t.Fatal("expected observation update")
	}
	if !obsStore.updated.StartedAt.Equal(existingStartedAt) {
		t.Fatalf("started_at = %v, want %v", obsStore.updated.StartedAt, existingStartedAt)
	}
}

func TestObservationFunctions_UpsertObservationPersistsIdentitySideEffects(t *testing.T) {
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
		ObservationID: "obs_upsert",
		SourceAssetID: "asset_001",
		StartedAt:     startedAt,
		JSON:          []byte(`{"identity":{"kind":"vehicle"}}`),
	}
	if err := f.UpsertObservation(context.Background(), obs); err != nil {
		t.Fatalf("UpsertObservation failed: %v", err)
	}
	if obsStore.upsert == nil {
		t.Fatal("expected upsert")
	}
	if obsStore.updated == nil {
		t.Fatal("expected follow-up update for identity side effects")
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"history_object_id"`)) {
		t.Fatalf("expected history_object_id in JSON, got %s", string(obsStore.updated.JSON))
	}
	if countAppendedFilename(objectGateway.appended, ObservationHistoryFilename) != 1 {
		t.Fatalf("expected one identity_patch history append, got %d", countAppendedFilename(objectGateway.appended, ObservationHistoryFilename))
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

func TestObservationFunctions_UpsertObservationRejectsLatestTelemetryOnUpdate(t *testing.T) {
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	telemetryJSON := []byte(`{"latest_telemetry":{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}}`)
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID:     "obs_001",
				SourceAssetID:     "asset_001",
				StartedAt:         startedAt,
				Version:           1,
				JSON:              telemetryJSON,
				LatestTelemetryAt: ptrTime(mustParseTime(t, "2026-01-01T00:06:00Z")),
			},
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator())
	update := *obsStore.byID["obs_001"]
	update.JSON = append([]byte(nil), telemetryJSON...)
	err := f.UpsertObservation(context.Background(), &update)
	if err == nil {
		t.Fatal("expected error for latest_telemetry on upsert update")
	}
}

func TestObservationFunctions_UpsertObservationPreservesLatestTelemetryWhenOmitted(t *testing.T) {
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	telemetryJSON := []byte(`{"latest_telemetry":{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}},"history_object_id":"obj_hist"}`)
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID:     "obs_001",
				SourceAssetID:     "asset_001",
				StartedAt:         startedAt,
				Version:           1,
				JSON:              telemetryJSON,
				LatestTelemetryAt: ptrTime(mustParseTime(t, "2026-01-01T00:06:00Z")),
			},
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator())
	endedAt := mustParseTime(t, "2026-01-02T00:00:00Z")
	update := *obsStore.byID["obs_001"]
	update.EndedAt = &endedAt
	update.JSON = []byte(`{"identity":{"kind":"asset"}}`)
	if err := f.UpsertObservation(context.Background(), &update); err != nil {
		t.Fatalf("UpsertObservation failed: %v", err)
	}
	if obsStore.upsert == nil {
		t.Fatal("expected upsert")
	}
	if !bytes.Contains(obsStore.upsert.JSON, []byte(`"latest_telemetry"`)) {
		t.Fatalf("expected latest_telemetry merged from existing, got %s", string(obsStore.upsert.JSON))
	}
}

func TestObservationFunctions_CreateObservationRejectsClientHistoryObjectID(t *testing.T) {
	f := NewObservationFunctions(&captureObservationStore{}, testLogger(), testProtoValidator())
	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     time.Now().UTC(),
		JSON:          []byte(`{"identity":{"kind":"asset"},"history_object_id":"obj_hist"}`),
	}
	err := f.CreateObservation(context.Background(), obs)
	if err == nil {
		t.Fatal("expected error for client history_object_id on create")
	}
	fieldErr, ok := err.(*model.FieldError)
	if !ok || fieldErr.Field != "json.history_object_id" {
		t.Fatalf("expected field error on json.history_object_id, got %T: %v", err, err)
	}
}

func TestObservationFunctions_UpsertObservationRejectsClientHistoryObjectID(t *testing.T) {
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID: "obs_001",
				SourceAssetID: "asset_001",
				StartedAt:     startedAt,
				Version:       1,
				JSON:          testObservationJSON,
			},
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator())
	update := *obsStore.byID["obs_001"]
	update.JSON = []byte(`{"identity":{"kind":"asset"},"history_object_id":"obj_hist"}`)
	err := f.UpsertObservation(context.Background(), &update)
	if err == nil {
		t.Fatal("expected error for client history_object_id on upsert")
	}
}

func TestObservationFunctions_UpsertObservationDefersIdentityUntilHistorySync(t *testing.T) {
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID: "obs_001",
				SourceAssetID: "asset_001",
				StartedAt:     startedAt,
				Version:       1,
				JSON:          testObservationJSON,
			},
		},
	}
	objectStore := &fakeObjectStore{
		getFn: func(_ context.Context, objectID string) (*model.Object, error) {
			return &model.Object{ObjectID: objectID, Type: model.ObjectTypeObservationHistory, OwnerType: model.OwnerTypeObservation, OwnerID: "obs_001"}, nil
		},
		createFn: func(_ context.Context, _ *model.Object) error { return nil },
		updateFn: func(_ context.Context, _ *model.Object) error { return nil },
	}
	objectGateway := &fakeObjectGateway{ObjectStore: objectStore}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator()).WithObjectGateway(objectGateway)
	newIdentity := []byte(`{"kind":"vehicle","vehicle_type":"sedan"}`)
	update := *obsStore.byID["obs_001"]
	merged, err := mergeObservationJSON(testObservationJSON, map[string]any{"identity": json.RawMessage(newIdentity)})
	if err != nil {
		t.Fatalf("mergeObservationJSON: %v", err)
	}
	update.JSON = merged
	if err := f.UpsertObservation(context.Background(), &update); err != nil {
		t.Fatalf("UpsertObservation failed: %v", err)
	}
	if obsStore.upsert == nil {
		t.Fatal("expected upsert")
	}
	if observationJSONHasKey(obsStore.upsert.JSON, "identity") {
		t.Fatalf("expected first upsert write without identity, got %s", string(obsStore.upsert.JSON))
	}
	if obsStore.updated == nil || !bytes.Contains(obsStore.updated.JSON, []byte(`"identity"`)) {
		t.Fatalf("expected identity after history sync, got upsert=%s updated=%v", string(obsStore.upsert.JSON), obsStore.updated)
	}
}

func TestObservationFunctions_UpdateObservationAllowsUnchangedLatestTelemetryAfterIngest(t *testing.T) {
	f, obsStore, _, _ := observationIngestTestFixtures(t)
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	ingested, err := f.IngestObservationTelemetry(context.Background(), ObservationTelemetryIngest{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		TelemetryJSON: []byte(`{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}`),
		StartedAt:     startedAt,
	})
	if err != nil {
		t.Fatalf("IngestObservationTelemetry failed: %v", err)
	}

	endedAt := mustParseTime(t, "2026-01-02T00:00:00Z")
	update := *ingested
	update.EndedAt = &endedAt
	update.JSON = []byte(`{"identity":{"kind":"asset"}}`)
	if err := f.UpdateObservation(context.Background(), &update); err != nil {
		t.Fatalf("UpdateObservation failed: %v", err)
	}
	if obsStore.updated == nil {
		t.Fatal("expected latest_telemetry preserved, got nil updated observation")
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"latest_telemetry"`)) {
		t.Fatalf("expected latest_telemetry preserved, got %s", string(obsStore.updated.JSON))
	}
	if obsStore.updated.EndedAt == nil || !obsStore.updated.EndedAt.Equal(endedAt) {
		t.Fatalf("expected ended_at update, got %v", obsStore.updated.EndedAt)
	}
}

func TestObservationFunctions_UpdateObservationRejectsChangedLatestTelemetry(t *testing.T) {
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	telemetryJSON := []byte(`{"latest_telemetry":{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}}`)
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID:     "obs_001",
				SourceAssetID:     "asset_001",
				StartedAt:         startedAt,
				Version:           1,
				JSON:              telemetryJSON,
				LatestTelemetryAt: ptrTime(mustParseTime(t, "2026-01-01T00:06:00Z")),
			},
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator())
	changedJSON := []byte(`{"latest_telemetry":{"observed_at":"2026-01-01T00:07:00Z","kind":"point","data":{"latitude":41.0,"longitude":-74.0}}}`)
	update := *obsStore.byID["obs_001"]
	update.JSON = changedJSON
	err := f.UpdateObservation(context.Background(), &update)
	if err == nil {
		t.Fatal("expected error for changed latest_telemetry")
	}
}

func TestObservationFunctions_UpdateObservationPreservesLatestTelemetryWhenOmitted(t *testing.T) {
	startedAt := mustParseTime(t, "2026-01-01T00:00:00Z")
	telemetryJSON := []byte(`{"latest_telemetry":{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}},"history_object_id":"obj_hist"}`)
	obsStore := &captureObservationStore{
		byID: map[string]*model.Observation{
			"obs_001": {
				ObservationID:     "obs_001",
				SourceAssetID:     "asset_001",
				StartedAt:         startedAt,
				Version:           1,
				JSON:              telemetryJSON,
				LatestTelemetryAt: ptrTime(mustParseTime(t, "2026-01-01T00:06:00Z")),
			},
		},
	}
	f := NewObservationFunctions(obsStore, testLogger(), testProtoValidator())
	endedAt := mustParseTime(t, "2026-01-02T00:00:00Z")
	update := *obsStore.byID["obs_001"]
	update.EndedAt = &endedAt
	update.JSON = []byte(`{"identity":{"kind":"asset"}}`)
	if err := f.UpdateObservation(context.Background(), &update); err != nil {
		t.Fatalf("UpdateObservation failed: %v", err)
	}
	if obsStore.updated == nil {
		t.Fatal("expected observation update")
	}
	if !bytes.Contains(obsStore.updated.JSON, []byte(`"latest_telemetry"`)) {
		t.Fatalf("expected latest_telemetry merged from existing, got %s", string(obsStore.updated.JSON))
	}
}

func TestObservationJSONObjectHelpers_RejectJSONNullRoot(t *testing.T) {
	if _, err := observationJSONPatchMap([]byte("null")); err == nil {
		t.Fatal("expected error for json null root in observationJSONPatchMap")
	}
	if _, err := mergeObservationJSON([]byte("null"), map[string]any{"identity": json.RawMessage(`{"kind":"asset"}`)}); err == nil {
		t.Fatal("expected error for json null root in mergeObservationJSON")
	}
	obs := &model.Observation{JSON: []byte("null")}
	if err := applyIdentityPatchToObservation(obs, json.RawMessage(`{"kind":"asset"}`), time.Now().UTC(), "obj_hist"); err == nil {
		t.Fatal("expected error for json null root in applyIdentityPatchToObservation")
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
