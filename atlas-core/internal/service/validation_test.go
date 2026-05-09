package function

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/anomalyco/atlas-core/internal/model"
)

func TestEntityWritePathsRejectInvalidJSONBeforeStore(t *testing.T) {
	ctx := context.Background()
	createCalls := 0
	updateCalls := 0
	upsertCalls := 0
	funcs := NewEntityFunctions(fakeEntityStore{
		createFn: func(context.Context, *model.Entity) error { createCalls++; return nil },
		updateFn: func(context.Context, *model.Entity) error { updateCalls++; return nil },
		upsertFn: func(context.Context, *model.Entity) error { upsertCalls++; return nil },
	}, testLogger())

	if err := funcs.CreateEntity(ctx, &model.Entity{EntityID: "asset-1", Type: model.EntityTypeAsset, JSON: []byte(`{"extra":{}}`)}); err == nil {
		t.Fatal("expected CreateEntity validation failure")
	}
	if err := funcs.UpdateEntity(ctx, &model.Entity{EntityID: "asset-1", Type: model.EntityTypeAsset, JSON: []byte(`{"extra":{}}`)}); err == nil {
		t.Fatal("expected UpdateEntity validation failure")
	}
	if err := funcs.UpsertEntity(ctx, &model.Entity{EntityID: "asset-1", Type: model.EntityTypeAsset, JSON: []byte(`{"extra":{}}`)}); err == nil {
		t.Fatal("expected UpsertEntity validation failure")
	}
	if createCalls != 0 || updateCalls != 0 || upsertCalls != 0 {
		t.Fatalf("expected no entity store calls, got create=%d update=%d upsert=%d", createCalls, updateCalls, upsertCalls)
	}
}

func TestObjectWritePathsRejectInvalidJSONBeforeStore(t *testing.T) {
	ctx := context.Background()
	createCalls := 0
	updateCalls := 0
	upsertCalls := 0
	pg := &fakeObjectStore{
		createFn: func(context.Context, *model.Object) error { createCalls++; return nil },
		updateFn: func(context.Context, *model.Object) error { updateCalls++; return nil },
		upsertFn: func(context.Context, *model.Object) error { upsertCalls++; return nil },
		getFn:    func(context.Context, string) (*model.Object, error) { return nil, model.ErrNotFound },
	}
	funcs := NewObjectFunctions(pg, fakeObjectStorage{}, fakeIdempotencyStore{}, testLogger())
	bad := &model.Object{ObjectID: "obj-1", Type: model.ObjectTypeDocument, OwnerType: model.OwnerTypeSystem, OwnerID: "system", JSON: []byte(`{"manifest":{}}`)}
	if err := funcs.CreateObject(ctx, bad); err == nil {
		t.Fatal("expected CreateObject validation failure")
	}
	if err := funcs.UpdateObject(ctx, bad); err == nil {
		t.Fatal("expected UpdateObject validation failure")
	}
	if err := funcs.UpsertObject(ctx, bad); err == nil {
		t.Fatal("expected UpsertObject validation failure")
	}
	if createCalls != 0 || updateCalls != 0 || upsertCalls != 0 {
		t.Fatalf("expected no object store calls, got create=%d update=%d upsert=%d", createCalls, updateCalls, upsertCalls)
	}
}

func TestTaskWritePathsRejectInvalidJSONBeforeStore(t *testing.T) {
	ctx := context.Background()
	createCalls := 0
	updateCalls := 0
	upsertCalls := 0
	funcs := NewTaskFunctions(fakeTaskStore{
		createFn: func(context.Context, *model.Task) error { createCalls++; return nil },
		updateFn: func(context.Context, *model.Task) error { updateCalls++; return nil },
		upsertFn: func(context.Context, *model.Task) error { upsertCalls++; return nil },
	}, fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset-1", Type: model.EntityTypeAsset, JSON: validAssetJSON()}, nil
	}}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return validCommandCatalogObject(), nil
	}}, fakeIdempotencyStore{}, testLogger())

	bad := &model.Task{TaskID: "task-1", Status: model.TaskStatusPending, AssetID: "asset-1", CommandCatalogObjectID: "command_catalog", JSON: []byte(`[]`)}
	if err := funcs.CreateTask(ctx, bad); err == nil {
		t.Fatal("expected CreateTask validation failure")
	}
	if err := funcs.UpdateTask(ctx, bad); err == nil {
		t.Fatal("expected UpdateTask validation failure")
	}
	if err := funcs.UpsertTask(ctx, bad); err == nil {
		t.Fatal("expected UpsertTask validation failure")
	}
	if createCalls != 0 || updateCalls != 0 || upsertCalls != 0 {
		t.Fatalf("expected no task store calls, got create=%d update=%d upsert=%d", createCalls, updateCalls, upsertCalls)
	}
}

func TestTaskCreateRejectsUnsupportedCommandBeforeStore(t *testing.T) {
	createCalls := 0
	funcs := NewTaskFunctions(fakeTaskStore{createFn: func(context.Context, *model.Task) error { createCalls++; return nil }}, fakeEntityStore{getFn: func(context.Context, string) (*model.Entity, error) {
		return &model.Entity{EntityID: "asset-1", Type: model.EntityTypeAsset, JSON: []byte(`{"components":{"supported_commands":{"commands":["hold_position"]}},"extra":{}}`)}, nil
	}}, &fakeObjectStore{getFn: func(context.Context, string) (*model.Object, error) {
		return validCommandCatalogObject(), nil
	}}, fakeIdempotencyStore{}, testLogger())

	err := funcs.CreateTask(context.Background(), &model.Task{TaskID: "task-1", Status: model.TaskStatusPending, AssetID: "asset-1", CommandCatalogObjectID: "command_catalog", JSON: validTaskJSON()})
	if err == nil {
		t.Fatal("expected unsupported command failure")
	}
	if createCalls != 0 {
		t.Fatalf("expected no task store call, got %d", createCalls)
	}
}

func TestObservationWritePathsRejectInvalidJSONBeforeStore(t *testing.T) {
	ctx := context.Background()
	createCalls := 0
	updateCalls := 0
	upsertCalls := 0
	funcs := NewObservationFunctions(fakeObservationStore{
		createFn: func(context.Context, *model.Observation) error { createCalls++; return nil },
		updateFn: func(context.Context, *model.Observation) error { updateCalls++; return nil },
		upsertFn: func(context.Context, *model.Observation) error { upsertCalls++; return nil },
	}, testLogger())

	bad := &model.Observation{ObservationID: "obs-1", SourceAssetID: "asset-1", JSON: []byte(`{"extra":{}}`)}
	if err := funcs.CreateObservation(ctx, bad); err == nil {
		t.Fatal("expected CreateObservation validation failure")
	}
	if err := funcs.UpdateObservation(ctx, bad); err == nil {
		t.Fatal("expected UpdateObservation validation failure")
	}
	if err := funcs.UpsertObservation(ctx, bad); err == nil {
		t.Fatal("expected UpsertObservation validation failure")
	}
	if createCalls != 0 || updateCalls != 0 || upsertCalls != 0 {
		t.Fatalf("expected no observation store calls, got create=%d update=%d upsert=%d", createCalls, updateCalls, upsertCalls)
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

func TestModelErrors_IsCoreError(t *testing.T) {
	wrapped := fmt.Errorf("wrapped: %w", model.ErrNotFound)
	if !errors.Is(wrapped, model.ErrNotFound) {
		t.Fatal("expected wrapped core error to match with errors.Is")
	}
}
