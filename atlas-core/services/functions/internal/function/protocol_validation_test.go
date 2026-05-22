package function

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"atlas.local/protocol"

	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

func TestEntityFunctions_InvalidEntityJSONRejectedBeforeStore(t *testing.T) {
	es := &entityStoreNoWrite{t: t}
	ef := NewEntityFunctions(es, testLogger(), testProtoValidator())

	entity := &model.Entity{
		EntityID: "ent_001",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`{"components":{"supported_commands":null}}`),
	}
	err := ef.CreateEntity(context.Background(), entity)
	if err == nil {
		t.Fatal("expected protocol validation error for invalid entity JSON")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestEntityFunctions_NilJSONNormalizedBeforeProtocolValidation(t *testing.T) {
	cases := []struct {
		name string
		call func(EntityFunctions, *model.Entity) error
	}{
		{
			name: "update",
			call: func(f EntityFunctions, entity *model.Entity) error {
				return f.UpdateEntity(context.Background(), entity)
			},
		},
		{
			name: "upsert",
			call: func(f EntityFunctions, entity *model.Entity) error {
				return f.UpsertEntity(context.Background(), entity)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			es := &entityStoreNoWrite{t: t}
			ef := NewEntityFunctions(es, testLogger(), testProtoValidator())

			entity := &model.Entity{
				EntityID: "ent_001",
				Type:     model.EntityTypeAsset,
			}

			err := tc.call(ef, entity)
			if err == nil {
				t.Fatal("expected protocol validation error")
			}
			if string(entity.JSON) != "{}" {
				t.Fatalf("expected nil JSON to be normalized to {}, got %q", string(entity.JSON))
			}

			var verr *protocolvalidation.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
			if hasIssueCode(verr.Issues, "invalid_json") {
				t.Fatalf("expected normalized JSON to avoid invalid_json issues, got %+v", verr.Issues)
			}
		})
	}
}

func TestEntityFunctions_InvalidEntityJSONRejectedBeforeStore_Update(t *testing.T) {
	es := &entityStoreNoWrite{t: t}
	ef := NewEntityFunctions(es, testLogger(), testProtoValidator())

	entity := &model.Entity{
		EntityID: "ent_001",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`not json`),
	}
	err := ef.UpdateEntity(context.Background(), entity)
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestEntityFunctions_InvalidEntityJSONRejectedBeforeStore_Upsert(t *testing.T) {
	es := &entityStoreNoWrite{t: t}
	ef := NewEntityFunctions(es, testLogger(), testProtoValidator())

	entity := &model.Entity{
		EntityID: "ent_001",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`not json`),
	}
	err := ef.UpsertEntity(context.Background(), entity)
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestObjectFunctions_InvalidObjectJSONRejectedBeforeStore(t *testing.T) {
	os := &objectStoreNoWrite{t: t}
	of := newTestObjectFunctions(os, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	obj := &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`not json`),
	}
	err := of.CreateObject(context.Background(), obj)
	if err == nil {
		t.Fatal("expected protocol validation error for invalid object JSON")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestObjectFunctions_InvalidObjectJSONRejectedBeforeStore_Update(t *testing.T) {
	os := &objectStoreNoWrite{t: t}
	of := newTestObjectFunctions(os, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	obj := &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`not json`),
	}
	err := of.UpdateObject(context.Background(), obj)
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestObjectFunctions_InvalidObjectJSONRejectedBeforeStore_Upsert(t *testing.T) {
	os := &objectStoreNoWrite{t: t}
	of := newTestObjectFunctions(os, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	obj := &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`not json`),
	}
	err := of.UpsertObject(context.Background(), obj)
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestObjectFunctions_InvalidCommandCatalogJSONRejectedBeforeStore(t *testing.T) {
	os := &objectStoreNoWrite{t: t}
	of := newTestObjectFunctions(os, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	obj := &model.Object{
		ObjectID:  "cmd_catalog",
		Type:      model.ObjectTypeCommandCatalog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`not json`),
	}
	err := of.CreateObject(context.Background(), obj)
	if err == nil {
		t.Fatal("expected protocol validation error for invalid command catalog JSON")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestObjectFunctions_CompletedIdempotencyReplaySkipsProtocolValidation(t *testing.T) {
	os := &objectStoreNoWrite{t: t}
	of := newTestObjectFunctions(os, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "obj_001", Status: store.IdempotencyStatusCompleted}, false, nil
		},
	}, testLogger(), testProtoValidator())

	obj := &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`not json`),
	}
	if err := of.CreateObject(context.Background(), obj, WithIdempotencyKey("client-1")); err != nil {
		t.Fatalf("expected completed replay to return nil before protocol validation, got %v", err)
	}
}

func TestTaskFunctions_InvalidTaskJSONRejectedBeforeStore(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{}}]}`)}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`)}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`not json`),
	}
	err := tf.CreateTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected protocol validation error for invalid task JSON")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskFunctions_InvalidTaskJSONRejectedBeforeStore_Update(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{}}]}`)}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`)}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`not json`),
	}
	err := tf.UpdateTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskFunctions_InvalidTaskJSONRejectedBeforeStore_Upsert(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{}}]}`)}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["test_cmd"]}}}`)}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`not json`),
	}
	err := tf.UpsertTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskFunctions_CompletedIdempotencyReplaySkipsRuntimeValidation(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &objectStoreNoWrite{t: t}
	es := &entityStoreNoWrite{t: t}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
			return store.IdempotencyRecord{ResourceID: "task_001", Status: store.IdempotencyStatusCompleted}, false, nil
		},
	}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_missing",
		CommandCatalogObjectID: "cmd_missing",
		JSON:                   []byte(`not json`),
	}
	if err := tf.CreateTask(context.Background(), task, WithIdempotencyKey("client-1")); err != nil {
		t.Fatalf("expected completed replay to return nil before runtime validation, got %v", err)
	}
}

func TestObservationFunctions_InvalidObservationJSONRejectedBeforeStore(t *testing.T) {
	os := &observationStoreNoWrite{t: t}
	of := NewObservationFunctions(os, testLogger(), testProtoValidator())

	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     time.Now().UTC(),
		JSON:          []byte(`not json`),
	}
	err := of.CreateObservation(context.Background(), obs)
	if err == nil {
		t.Fatal("expected protocol validation error for invalid observation JSON")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestObservationFunctions_InvalidObservationJSONRejectedBeforeStore_Update(t *testing.T) {
	os := &observationStoreNoWrite{t: t}
	of := NewObservationFunctions(os, testLogger(), testProtoValidator())

	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     time.Now().UTC(),
		JSON:          []byte(`not json`),
	}
	err := of.UpdateObservation(context.Background(), obs)
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestObservationFunctions_InvalidObservationJSONRejectedBeforeStore_Upsert(t *testing.T) {
	os := &observationStoreNoWrite{t: t}
	of := NewObservationFunctions(os, testLogger(), testProtoValidator())

	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     time.Now().UTC(),
		JSON:          []byte(`not json`),
	}
	err := of.UpsertObservation(context.Background(), obs)
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestObservationFunctions_EmptyJSONRejectedBeforeProtocolValidation(t *testing.T) {
	cases := []struct {
		name string
		call func(ObservationFunctions, *model.Observation) error
	}{
		{
			name: "create",
			call: func(f ObservationFunctions, obs *model.Observation) error {
				return f.CreateObservation(context.Background(), obs)
			},
		},
		{
			name: "update",
			call: func(f ObservationFunctions, obs *model.Observation) error {
				return f.UpdateObservation(context.Background(), obs)
			},
		},
		{
			name: "upsert",
			call: func(f ObservationFunctions, obs *model.Observation) error {
				return f.UpsertObservation(context.Background(), obs)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+" nil json", func(t *testing.T) {
			os := &observationStoreNoWrite{t: t}
			of := NewObservationFunctions(os, testLogger(), testProtoValidator())

			obs := &model.Observation{
				ObservationID: "obs_001",
				SourceAssetID: "asset_001",
				StartedAt:     time.Now().UTC(),
			}

			err := tc.call(of, obs)
			if err == nil {
				t.Fatal("expected validation error for nil json")
			}
			fieldErr, ok := err.(*model.FieldError)
			if !ok || fieldErr.Field != "json" {
				t.Fatalf("expected field error on json, got %T: %v", err, err)
			}
		})

		t.Run(tc.name+" empty object", func(t *testing.T) {
			os := &observationStoreNoWrite{t: t}
			of := NewObservationFunctions(os, testLogger(), testProtoValidator())

			obs := &model.Observation{
				ObservationID: "obs_001",
				SourceAssetID: "asset_001",
				StartedAt:     time.Now().UTC(),
				JSON:          []byte(`{ }`),
			}

			err := tc.call(of, obs)
			if err == nil {
				t.Fatal("expected validation error for empty json object")
			}
			fieldErr, ok := err.(*model.FieldError)
			if !ok || fieldErr.Field != "json" {
				t.Fatalf("expected field error on json, got %T: %v", err, err)
			}
		})
	}
}

func TestObservationFunctions_RejectedObservationJSONUsesProtocolValidation(t *testing.T) {
	os := &observationStoreNoWrite{t: t}
	of := NewObservationFunctions(os, testLogger(), testProtoValidator())

	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "asset_001",
		StartedAt:     time.Now().UTC(),
		JSON:          []byte(`{"state":"active"}`),
	}
	err := of.CreateObservation(context.Background(), obs)
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestProtocolIssues_PreserveFieldCodeMessage(t *testing.T) {
	es := &entityStoreNoWrite{t: t}
	ef := NewEntityFunctions(es, testLogger(), testProtoValidator())

	entity := &model.Entity{
		EntityID: "ent_001",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`{"components":{"supported_commands":null}}`),
	}
	err := ef.CreateEntity(context.Background(), entity)
	if err == nil {
		t.Fatal("expected error")
	}
	var verr *protocolvalidation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if len(verr.Issues) == 0 {
		t.Fatal("expected at least one validation issue")
	}
	sorted := sortIssues(verr.Issues)
	for _, issue := range sorted {
		if issue.Field == "" {
			t.Fatal("expected non-empty field in protocol issue")
		}
		if issue.Code == "" {
			t.Fatal("expected non-empty code in protocol issue")
		}
		if issue.Message == "" {
			t.Fatal("expected non-empty message in protocol issue")
		}
	}
}

func sortIssues(issues []protocol.ValidationIssue) []protocol.ValidationIssue {
	sorted := make([]protocol.ValidationIssue, len(issues))
	copy(sorted, issues)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Field != sorted[j].Field {
			return sorted[i].Field < sorted[j].Field
		}
		if sorted[i].Code != sorted[j].Code {
			return sorted[i].Code < sorted[j].Code
		}
		return sorted[i].Message < sorted[j].Message
	})
	return sorted
}

func hasIssueCode(issues []protocol.ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

// --- No-write stores that panic if called ---

type entityStoreNoWrite struct {
	t *testing.T
}

func (s *entityStoreNoWrite) CreateEntity(ctx context.Context, entity *model.Entity) error {
	s.t.Fatal("CreateEntity should not be called after protocol validation failure")
	return nil
}
func (s *entityStoreNoWrite) GetEntity(ctx context.Context, entityID string) (*model.Entity, error) {
	s.t.Fatal("GetEntity should not be called")
	return nil, nil
}
func (s *entityStoreNoWrite) ListEntities(context.Context, store.EntityListParams) (store.EntityListResult, error) {
	return store.EntityListResult{}, nil
}
func (s *entityStoreNoWrite) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	s.t.Fatal("UpdateEntity should not be called after protocol validation failure")
	return nil
}
func (s *entityStoreNoWrite) DeleteEntity(ctx context.Context, entityID string) error {
	return nil
}
func (s *entityStoreNoWrite) UpsertEntity(ctx context.Context, entity *model.Entity) error {
	s.t.Fatal("UpsertEntity should not be called after protocol validation failure")
	return nil
}

type objectStoreNoWrite struct {
	t *testing.T
}

func (s *objectStoreNoWrite) CreateObject(ctx context.Context, obj *model.Object) error {
	s.t.Fatal("CreateObject should not be called after protocol validation failure")
	return nil
}
func (s *objectStoreNoWrite) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	return nil, model.ErrNotFound
}
func (s *objectStoreNoWrite) ListObjects(context.Context, store.ObjectListParams) (store.ObjectListResult, error) {
	return store.ObjectListResult{}, nil
}
func (s *objectStoreNoWrite) UpdateObject(ctx context.Context, obj *model.Object) error {
	s.t.Fatal("UpdateObject should not be called after protocol validation failure")
	return nil
}
func (s *objectStoreNoWrite) DeleteObject(ctx context.Context, objectID string) error {
	return nil
}
func (s *objectStoreNoWrite) UpsertObject(ctx context.Context, obj *model.Object) error {
	s.t.Fatal("UpsertObject should not be called after protocol validation failure")
	return nil
}
func (s *objectStoreNoWrite) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest, updatedAt ...time.Time) error {
	return nil
}
func (s *objectStoreNoWrite) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	return model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}), nil
}

type taskStoreNoWrite struct {
	t *testing.T
}

func (s *taskStoreNoWrite) CreateTask(ctx context.Context, task *model.Task) error {
	s.t.Fatal("CreateTask should not be called after protocol validation failure")
	return nil
}
func (s *taskStoreNoWrite) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	return nil, model.ErrNotFound
}
func (s *taskStoreNoWrite) ListTasks(context.Context, store.TaskListParams) (store.TaskListResult, error) {
	return store.TaskListResult{}, nil
}
func (s *taskStoreNoWrite) UpdateTask(ctx context.Context, task *model.Task) error {
	s.t.Fatal("UpdateTask should not be called after protocol validation failure")
	return nil
}
func (s *taskStoreNoWrite) DeleteTask(ctx context.Context, taskID string) error {
	return nil
}
func (s *taskStoreNoWrite) UpsertTask(ctx context.Context, task *model.Task) error {
	s.t.Fatal("UpsertTask should not be called after protocol validation failure")
	return nil
}

type observationStoreNoWrite struct {
	t *testing.T
}

func (s *observationStoreNoWrite) CreateObservation(ctx context.Context, obs *model.Observation) error {
	s.t.Fatal("CreateObservation should not be called after protocol validation failure")
	return nil
}
func (s *observationStoreNoWrite) GetObservation(ctx context.Context, observationID string) (*model.Observation, error) {
	return &model.Observation{
		ObservationID: observationID,
		SourceAssetID: "asset_001",
		StartedAt:     time.Now().UTC(),
		JSON:          testObservationJSON,
		Version:       1,
	}, nil
}
func (s *observationStoreNoWrite) ListObservations(context.Context, store.ObservationListParams) (store.ObservationListResult, error) {
	return store.ObservationListResult{}, nil
}
func (s *observationStoreNoWrite) UpdateObservation(ctx context.Context, obs *model.Observation) error {
	s.t.Fatal("UpdateObservation should not be called after protocol validation failure")
	return nil
}
func (s *observationStoreNoWrite) DeleteObservation(ctx context.Context, observationID string) error {
	return nil
}
func (s *observationStoreNoWrite) UpsertObservation(ctx context.Context, obs *model.Observation) error {
	s.t.Fatal("UpsertObservation should not be called after protocol validation failure")
	return nil
}
