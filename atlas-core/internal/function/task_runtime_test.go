package function

import (
	"context"
	"errors"
	"testing"

	"github.com/anomalyco/atlas-core/internal/model"
)

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
	if err == nil {
		t.Fatal("expected error for missing target asset")
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
	if err == nil {
		t.Fatal("expected error for target that is not an asset")
	}
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
	if err == nil {
		t.Fatal("expected error for unsupported command")
	}
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
	if err == nil {
		t.Fatal("expected error for missing command catalog")
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
	if err == nil {
		t.Fatal("expected error for command missing from catalog")
	}
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
	if err == nil {
		t.Fatal("expected error for missing required parameter")
	}
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
	if err == nil {
		t.Fatal("expected error for invalid parameter type")
	}
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
	if err == nil {
		t.Fatal("expected error for unknown parameter")
	}
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected FieldError, got %T: %v", err, err)
	}
	if fieldErr.Field != "json.components.parameters.unexpected" {
		t.Fatalf("expected unexpected parameter field, got %s", fieldErr.Field)
	}
}
