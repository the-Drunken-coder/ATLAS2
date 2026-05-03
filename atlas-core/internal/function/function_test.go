package function

import (
	"context"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/model"
)

func TestEntityFunctions_ValidateEntityID(t *testing.T) {
	f := EntityFunctions{}

	entity := &model.Entity{
		EntityID:  "",
		Type:      model.EntityTypeAsset,
		JSON:      []byte(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := f.CreateEntity(nil, entity)
	if err == nil {
		t.Fatal("expected error for empty entity_id")
	}
	fe, ok := err.(*model.FieldError)
	if !ok || fe.Field != "entity_id" {
		t.Fatalf("expected FieldError on entity_id, got %T: %v", err, err)
	}

	entity.EntityID = "this-entity-id-is-way-too-long-for-the-50-character-limit"
	err = f.CreateEntity(nil, entity)
	if err == nil {
		t.Fatal("expected error for long entity_id")
	}
}

func TestEntityFunctions_ValidateType(t *testing.T) {
	f := EntityFunctions{}

	entity := &model.Entity{
		EntityID:  "test_001",
		Type:      model.EntityType("invalid_type"),
		JSON:      []byte(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := f.CreateEntity(nil, entity)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestObservationFunctions_ValidateObservationID(t *testing.T) {
	f := ObservationFunctions{}

	obs := &model.Observation{
		ObservationID: "",
		SourceAssetID: "asset_001",
		JSON:          []byte(`{}`),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	err := f.CreateObservation(nil, obs)
	if err == nil {
		t.Fatal("expected error for empty observation_id")
	}
}

func TestObservationFunctions_ValidateSourceAssetID(t *testing.T) {
	f := ObservationFunctions{}

	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "",
		JSON:          []byte(`{}`),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	err := f.CreateObservation(nil, obs)
	if err == nil {
		t.Fatal("expected error for empty source_asset_id")
	}
}

func TestTaskFunctions_ValidateTaskID(t *testing.T) {
	f := TaskFunctions{}

	task := &model.Task{
		TaskID:                 "",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`{}`),
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}
	err := f.CreateTask(nil, task)
	if err == nil {
		t.Fatal("expected error for empty task_id")
	}
}

func TestTaskFunctions_ValidateRequiredFields(t *testing.T) {
	f := TaskFunctions{}

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 "",
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`{}`),
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}
	err := f.CreateTask(nil, task)
	if err == nil {
		t.Fatal("expected error for empty status")
	}

	task.Status = model.TaskStatusPending
	task.AssetID = ""
	err = f.CreateTask(nil, task)
	if err == nil {
		t.Fatal("expected error for empty asset_id")
	}

	task.AssetID = "asset_001"
	task.CommandCatalogObjectID = ""
	err = f.CreateTask(nil, task)
	if err == nil {
		t.Fatal("expected error for empty command_catalog_object_id")
	}
}

func TestObjectFunctions_ValidateObjectID(t *testing.T) {
	f := ObjectFunctions{}

	obj := &model.Object{
		ObjectID:  "",
		Type:      "log",
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := f.CreateObject(nil, obj)
	if err == nil {
		t.Fatal("expected error for empty object_id")
	}
}

func TestObjectFunctions_ValidateRequiredFields(t *testing.T) {
	f := ObjectFunctions{}

	obj := &model.Object{
		ObjectID:  "obj_001",
		Type:      "",
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := f.CreateObject(nil, obj)
	if err == nil {
		t.Fatal("expected error for empty type")
	}

	obj.Type = "log"
	obj.OwnerType = ""
	err = f.CreateObject(nil, obj)
	if err == nil {
		t.Fatal("expected error for empty owner_type")
	}

	obj.OwnerType = model.OwnerTypeSystem
	obj.OwnerID = ""
	err = f.CreateObject(nil, obj)
	if err == nil {
		t.Fatal("expected error for empty owner_id")
	}
}

func TestFunctions_RejectNilModels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "entity", err: EntityFunctions{}.CreateEntity(context.Background(), nil)},
		{name: "object", err: ObjectFunctions{}.CreateObject(context.Background(), nil)},
		{name: "task", err: TaskFunctions{}.CreateTask(context.Background(), nil)},
		{name: "observation", err: ObservationFunctions{}.CreateObservation(context.Background(), nil)},
	}

	for _, tt := range tests {
		if tt.err == nil {
			t.Fatalf("expected error for nil %s model", tt.name)
		}
	}
}

func TestObjectFunctions_RejectUnsafeObjectID(t *testing.T) {
	f := ObjectFunctions{}

	obj := &model.Object{
		ObjectID:  "../obj",
		Type:      "log",
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
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

func TestModelErrors_IsCoreError(t *testing.T) {
	err := model.NewCoreError("TEST", "test message")
	if err.Error() != "TEST: test message" {
		t.Fatalf("unexpected error string: %s", err.Error())
	}

	fe := model.NewFieldError("TEST", "test", "field")
	if fe.Field != "field" {
		t.Fatalf("expected field 'field', got '%s'", fe.Field)
	}
}
