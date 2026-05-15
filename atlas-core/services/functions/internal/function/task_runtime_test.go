package function

import (
	"context"
	"errors"
	"testing"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

func requireFieldError(t *testing.T, err error, code, field string) *model.FieldError {
	t.Helper()
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected FieldError, got %T: %v", err, err)
	}
	if fieldErr.Code != code {
		t.Fatalf("expected code %s, got %s", code, fieldErr.Code)
	}
	if fieldErr.Field != field {
		t.Fatalf("expected field %s, got %s", field, fieldErr.Field)
	}
	return fieldErr
}

func validTaskJSON(cmd string) []byte {
	return []byte(`{"components":{"command":{"type":"` + cmd + `"},"parameters":{}}}`)
}

func validCatalogJSON(cmds ...string) []byte {
	entries := ""
	for i, cmd := range cmds {
		if i > 0 {
			entries += ","
		}
		entries += `{"id":"` + cmd + `","name":"Test","description":"Test","parameters_schema":{}}`
	}
	return []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[` + entries + `]}`)
}

func assetEntityJSON(commands ...string) []byte {
	list := ""
	for i, cmd := range commands {
		if i > 0 {
			list += ","
		}
		list += `"` + cmd + `"`
	}
	return []byte(`{"components":{"supported_commands":{"commands":[` + list + `]}}}`)
}

func TestTaskRuntime_MissingTargetAsset(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: validCatalogJSON("test_cmd")}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return nil, model.ErrNotFound
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_missing",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   validTaskJSON("test_cmd"),
	}
	err := tf.CreateTask(context.Background(), task)
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing target asset, got %v", err)
	}
}

func TestTaskRuntime_TargetNotAnAsset(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: validCatalogJSON("test_cmd")}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "track_001", Type: model.EntityTypeTrack, JSON: []byte(`{}`)}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "track_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   validTaskJSON("test_cmd"),
	}
	err := tf.CreateTask(context.Background(), task)
	requireFieldError(t, err, "INVALID_INPUT", "asset_id")
}

func TestTaskRuntime_UnsupportedCommand(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: validCatalogJSON("test_cmd")}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: assetEntityJSON("move_to_location")}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   validTaskJSON("test_cmd"),
	}
	err := tf.CreateTask(context.Background(), task)
	requireFieldError(t, err, "INVALID_INPUT", "json.components.command.type")
}

func TestTaskRuntime_MissingCommandCatalog(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return nil, model.ErrNotFound
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: assetEntityJSON("test_cmd")}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_missing",
		JSON:                   validTaskJSON("test_cmd"),
	}
	err := tf.CreateTask(context.Background(), task)
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing command catalog, got %v", err)
	}
}

func TestTaskRuntime_CommandMissingFromCatalog(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: validCatalogJSON("other_cmd")}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: assetEntityJSON("test_cmd")}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   validTaskJSON("test_cmd"),
	}
	err := tf.CreateTask(context.Background(), task)
	requireFieldError(t, err, "INVALID_INPUT", "json.components.command.type")
}

func TestTaskRuntime_MissingRequiredParameter(t *testing.T) {
	catalogJSON := []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{"lat":{"type":"number","required":true}}}]}`)
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: catalogJSON}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: assetEntityJSON("test_cmd")}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   validTaskJSON("test_cmd"),
	}
	err := tf.CreateTask(context.Background(), task)
	requireFieldError(t, err, "INVALID_INPUT", "json.components.parameters.lat")
}

func TestTaskRuntime_InvalidParameterType(t *testing.T) {
	catalogJSON := []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{"count":{"type":"number","required":true}}}]}`)
	taskJSON := []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{"count":"not_a_number"}}}`)
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: catalogJSON}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: assetEntityJSON("test_cmd")}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   taskJSON,
	}
	err := tf.CreateTask(context.Background(), task)
	requireFieldError(t, err, "INVALID_INPUT", "json.components.parameters.count")
}

func TestTaskRuntime_UnknownParameter(t *testing.T) {
	catalogJSON := []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[{"id":"test_cmd","name":"Test","description":"Test","parameters_schema":{"lat":{"type":"number","required":false}}}]}`)
	taskJSON := []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{"lat":1,"unexpected":true}}}`)
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: catalogJSON}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: assetEntityJSON("test_cmd")}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), testProtoValidator())

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   taskJSON,
	}
	err := tf.CreateTask(context.Background(), task)
	requireFieldError(t, err, "INVALID_INPUT", "json.components.parameters.unexpected")
}

func TestTaskRuntime_CorruptTaskJSON(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: validCatalogJSON("test_cmd")}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: assetEntityJSON("test_cmd")}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), fakeProtocolValidator{})

	err := tf.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   []byte(`{`),
	})
	fieldErr := requireFieldError(t, err, "INTERNAL", "json")
	if fieldErr.Message != "task JSON is corrupt" {
		t.Fatalf("expected corrupt task JSON message, got %q", fieldErr.Message)
	}
}

func TestTaskRuntime_CorruptCatalogJSON(t *testing.T) {
	ts := &taskStoreNoWrite{t: t}
	os := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "cmd_001", Type: model.ObjectTypeCommandCatalog, JSON: []byte(`{`)}, nil
	}}
	es := &fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: assetEntityJSON("test_cmd")}, nil
	}}
	tf := NewTaskFunctions(ts, os, es, fakeIdempotencyStore{}, testLogger(), fakeProtocolValidator{})

	err := tf.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "cmd_001",
		JSON:                   validTaskJSON("test_cmd"),
	})
	fieldErr := requireFieldError(t, err, "INTERNAL", "command_catalog_object_id")
	if fieldErr.Message != "command catalog JSON is corrupt" {
		t.Fatalf("expected corrupt catalog JSON message, got %q", fieldErr.Message)
	}
}
