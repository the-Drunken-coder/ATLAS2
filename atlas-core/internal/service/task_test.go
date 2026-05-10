package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/protocolvalidation"
)

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
	f := NewTaskFunctions(fakeTaskStore{}, fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog}, nil
	}}, fakeIdempotencyStore{}, testLogger())
	task := &model.Task{TaskID: "task_001", Status: model.TaskStatusPending, AssetID: "asset_001", CommandCatalogObjectID: "obj_001", JSON: validTaskJSON()}
	if err := f.CreateTask(context.Background(), task); err == nil {
		t.Fatal("expected task validation failure")
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
		return validCommandCatalogObject(), nil
	}}
	entityStore := fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}
	idem := fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (ports.IdempotencyRecord, bool, error) {
			return ports.IdempotencyRecord{ResourceID: "task_001", Status: ports.IdempotencyStatusPending}, false, nil
		},
		markCompletedFn: func(context.Context, string, string) error {
			completed = true
			return nil
		},
	}
	f := NewTaskFunctions(taskStore, entityStore, objectStore, idem, testLogger())

	if err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   validTaskJSON(),
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
		return validCommandCatalogObject(), nil
	}}
	entityStore := fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}
	f := NewTaskFunctions(taskStore, entityStore, objectStore, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (ports.IdempotencyRecord, bool, error) {
			return ports.IdempotencyRecord{ResourceID: "task_001", Status: ports.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger())

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   validTaskJSON(),
	}, WithIdempotencyKey("fresh-key"))
	if !errors.Is(err, model.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if !markedFailed {
		t.Fatal("expected fresh task idempotency claim to be marked failed on duplicate task")
	}
}

func TestTaskFunctions_CreateTaskValidatesParametersAgainstCommandCatalog(t *testing.T) {
	created := false
	f := NewTaskFunctions(fakeTaskStore{
		createFn: func(context.Context, *model.Task) error {
			created = true
			return nil
		},
	}, fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return validCommandCatalogObject(), nil
	}}, fakeIdempotencyStore{}, testLogger())

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   []byte(`{"components":{"command":{"type":"move_to_location"},"parameters":{"latitude":40,"mode":"auto"}}}`),
	})
	if err != nil {
		t.Fatalf("expected valid catalog-backed parameters to pass, got %v", err)
	}
	if !created {
		t.Fatal("expected task create after catalog validation")
	}
}

func TestTaskFunctions_CreateTaskRejectsParametersOutsideCommandCatalogSchema(t *testing.T) {
	f := NewTaskFunctions(fakeTaskStore{
		createFn: func(context.Context, *model.Task) error {
			t.Fatal("task store should not be called")
			return nil
		},
	}, fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return validCommandCatalogObject(), nil
	}}, fakeIdempotencyStore{}, testLogger())

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   []byte(`{"components":{"command":{"type":"move_to_location"},"parameters":{"mode":"bad","unexpected":true}}}`),
	})
	var validationErr *protocolvalidation.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected blob validation error, got %v", err)
	}
	fields := map[string]struct{}{}
	for _, violation := range validationErr.Violations {
		fields[violation.Field] = struct{}{}
	}
	for _, want := range []string{"json.components.parameters.mode", "json.components.parameters.unexpected"} {
		if _, ok := fields[want]; !ok {
			t.Fatalf("expected violation for %s, got %+v", want, validationErr.Violations)
		}
	}
}

func TestTaskFunctions_CreateTaskRejectsCommandMissingFromCatalog(t *testing.T) {
	f := NewTaskFunctions(fakeTaskStore{}, fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["move_to_location"]}},"extra":{}}`)}, nil
	}}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "command_catalog", Type: model.ObjectTypeDocument, JSON: []byte(`{"commands":{}}`)}, nil
	}}, fakeIdempotencyStore{}, testLogger())

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   validTaskJSON(),
	})
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "json.components.command.type" {
		t.Fatalf("expected command type field error, got %v", err)
	}
}

func TestTaskFunctions_CreateTaskRejectsCatalogCommandWithoutParametersSchema(t *testing.T) {
	f := NewTaskFunctions(fakeTaskStore{}, fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "command_catalog", Type: model.ObjectTypeDocument, JSON: []byte(`{"commands":{"move_to_location":{}}}`)}, nil
	}}, fakeIdempotencyStore{}, testLogger())

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   validTaskJSON(),
	})
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "command_catalog_object_id" {
		t.Fatalf("expected command catalog field error, got %v", err)
	}
}

func TestTaskFunctions_CreateTaskRejectsInvalidCatalogPayloadShape(t *testing.T) {
	f := NewTaskFunctions(fakeTaskStore{}, fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "command_catalog", Type: model.ObjectTypeDocument, JSON: []byte(`{"commands":[]}`)}, nil
	}}, fakeIdempotencyStore{}, testLogger())

	err := f.CreateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   validTaskJSON(),
	})
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected command catalog field error, got %v", err)
	}
	if fieldErr.Field != "command_catalog_object_id" {
		t.Fatalf("expected command catalog field error, got %v", err)
	}
	if !strings.Contains(fieldErr.Message, "commands object") {
		t.Fatalf("expected commands object error, got %q", fieldErr.Message)
	}
}

func TestTaskFunctions_UpdateTaskAllowsLegacyCommandCatalogReference(t *testing.T) {
	updated := false
	taskStore := fakeTaskStore{
		getFn: func(context.Context, string) (*model.Task, error) {
			return &model.Task{
				TaskID:                 "task_001",
				Status:                 model.TaskStatusPending,
				AssetID:                "asset_001",
				CommandCatalogObjectID: "legacy_catalog",
				JSON:                   validTaskJSON(),
			}, nil
		},
		updateFn: func(context.Context, *model.Task) error {
			updated = true
			return nil
		},
	}
	entityStore := fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}
	objectStore := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "legacy_catalog", Type: model.ObjectType("command_catalog")}, nil
	}}
	f := NewTaskFunctions(taskStore, entityStore, objectStore, fakeIdempotencyStore{}, testLogger())

	if err := f.UpdateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusCompleted,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "legacy_catalog",
		JSON:                   validTaskJSON(),
	}); err != nil {
		t.Fatalf("expected legacy task update to succeed, got %v", err)
	}
	if !updated {
		t.Fatal("expected update task store call")
	}
}

func TestTaskFunctions_UpdateTaskRejectsChangedParametersOnLegacyCommandCatalogReference(t *testing.T) {
	taskStore := fakeTaskStore{
		getFn: func(context.Context, string) (*model.Task, error) {
			return &model.Task{
				TaskID:                 "task_001",
				Status:                 model.TaskStatusPending,
				AssetID:                "asset_001",
				CommandCatalogObjectID: "legacy_catalog",
				JSON:                   validTaskJSON(),
			}, nil
		},
	}
	entityStore := fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}
	objectStore := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return &model.Object{ObjectID: "legacy_catalog", Type: model.ObjectType("command_catalog")}, nil
	}}
	f := NewTaskFunctions(taskStore, entityStore, objectStore, fakeIdempotencyStore{}, testLogger())

	mutated := []byte(`{"components":{"command":{"type":"move_to_location"},"parameters":{"latitude":1.5}},"extra":{}}`)
	err := f.UpdateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "legacy_catalog",
		JSON:                   mutated,
	})
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected command catalog field error, got %v", err)
	}
	if fieldErr.Field != "command_catalog_object_id" {
		t.Fatalf("expected command_catalog_object_id field error, got %v", fieldErr.Field)
	}
}

func TestTaskFunctions_UpdateTaskAllowsUnchangedCommandWhenCatalogWasDeleted(t *testing.T) {
	updated := false
	taskStore := fakeTaskStore{
		getFn: func(context.Context, string) (*model.Task, error) {
			return &model.Task{
				TaskID:                 "task_001",
				Status:                 model.TaskStatusPending,
				AssetID:                "asset_001",
				CommandCatalogObjectID: "command_catalog",
				JSON:                   validTaskJSON(),
			}, nil
		},
		updateFn: func(context.Context, *model.Task) error {
			updated = true
			return nil
		},
	}
	entityStore := fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}
	objectStore := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return nil, model.ErrNotFound
	}}
	f := NewTaskFunctions(taskStore, entityStore, objectStore, fakeIdempotencyStore{}, testLogger())

	if err := f.UpdateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusCompleted,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   validTaskJSON(),
	}); err != nil {
		t.Fatalf("expected update to succeed when command is unchanged, got %v", err)
	}
	if !updated {
		t.Fatal("expected update task store call")
	}
}

func TestTaskFunctions_UpdateTaskRejectsChangedParametersWhenCatalogWasDeleted(t *testing.T) {
	taskStore := fakeTaskStore{
		getFn: func(context.Context, string) (*model.Task, error) {
			return &model.Task{
				TaskID:                 "task_001",
				Status:                 model.TaskStatusPending,
				AssetID:                "asset_001",
				CommandCatalogObjectID: "command_catalog",
				JSON:                   validTaskJSON(),
			}, nil
		},
	}
	entityStore := fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset_001", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}
	objectStore := &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return nil, model.ErrNotFound
	}}
	f := NewTaskFunctions(taskStore, entityStore, objectStore, fakeIdempotencyStore{}, testLogger())

	err := f.UpdateTask(context.Background(), &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_001",
		CommandCatalogObjectID: "command_catalog",
		JSON:                   []byte(`{"components":{"command":{"type":"move_to_location"},"parameters":{"latitude":41}}}`),
	})
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) || fieldErr.Field != "command_catalog_object_id" {
		t.Fatalf("expected command catalog field error, got %v", err)
	}
}
