package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/store"
)

func TestObjectStore_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj := &model.Object{
		ObjectID:  "obj_001",
		Type:      "command_catalog",
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{"desc":"test"}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.CreateObject(ctx, obj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

	got, err := s.GetObject(ctx, "obj_001")
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	if got.ObjectID != "obj_001" {
		t.Fatalf("expected 'obj_001', got '%s'", got.ObjectID)
	}
}

func TestObjectStore_ListByOwner(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj1 := &model.Object{
		ObjectID: "o1", Type: "log", OwnerType: model.OwnerTypeEntity,
		OwnerID: "entity_a", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	obj2 := &model.Object{
		ObjectID: "o2", Type: "log", OwnerType: model.OwnerTypeEntity,
		OwnerID: "entity_b", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	s.CreateObject(ctx, obj1)
	s.CreateObject(ctx, obj2)

	results, err := s.ListObjects(ctx, store.WithObjectOwnerType(model.OwnerTypeEntity), store.WithObjectOwnerID("entity_a"))
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 object, got %d", len(results))
	}
	if results[0].ObjectID != "o1" {
		t.Fatalf("expected 'o1', got '%s'", results[0].ObjectID)
	}
}

func TestObjectStore_UpdateAndDelete(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj := &model.Object{
		ObjectID: "od1", Type: "log", OwnerType: model.OwnerTypeTask,
		OwnerID: "task_a", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	s.CreateObject(ctx, obj)

	obj.Type = "photo"
	obj.UpdatedAt = time.Now()
	if err := s.UpdateObject(ctx, obj); err != nil {
		t.Fatalf("UpdateObject failed: %v", err)
	}

	got, _ := s.GetObject(ctx, "od1")
	if got.Type != "photo" {
		t.Fatalf("expected type 'photo', got '%s'", got.Type)
	}

	if err := s.DeleteObject(ctx, "od1"); err != nil {
		t.Fatalf("DeleteObject failed: %v", err)
	}
}

func TestObjectStore_Upsert(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj := &model.Object{
		ObjectID: "ups_obj", Type: "log", OwnerType: model.OwnerTypeSystem,
		OwnerID: "sys", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := s.UpsertObject(ctx, obj); err != nil {
		t.Fatalf("UpsertObject insert failed: %v", err)
	}

	obj.Type = "photo"
	obj.UpdatedAt = time.Now()
	if err := s.UpsertObject(ctx, obj); err != nil {
		t.Fatalf("UpsertObject update failed: %v", err)
	}

	got, _ := s.GetObject(ctx, "ups_obj")
	if got.Type != "photo" {
		t.Fatalf("expected 'photo' after upsert, got '%s'", got.Type)
	}
}

func TestObjectStore_ListByType(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj1 := &model.Object{
		ObjectID: "lt1", Type: "log", OwnerType: model.OwnerTypeSystem,
		OwnerID: "sys", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	obj2 := &model.Object{
		ObjectID: "lt2", Type: "photo", OwnerType: model.OwnerTypeSystem,
		OwnerID: "sys", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	s.CreateObject(ctx, obj1)
	s.CreateObject(ctx, obj2)

	photos, _ := s.ListObjects(ctx, store.WithObjectType("photo"))
	if len(photos) != 1 {
		t.Fatalf("expected 1 photo, got %d", len(photos))
	}

	logs, _ := s.ListObjects(ctx, store.WithObjectType("log"))
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
}

func TestTaskStore_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	entityStore := NewEntityStore(pool)
	objectStore := NewObjectStore(pool)
	taskStore := NewTaskStore(pool)
	ctx := context.Background()

	// Setup required FK references
	asset := &model.Entity{
		EntityID: "asset_tsk", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	entityStore.CreateEntity(ctx, asset)

	catObj := &model.Object{
		ObjectID: "cmd_cat", Type: "command_catalog",
		OwnerType: model.OwnerTypeSystem, OwnerID: "system",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	objectStore.CreateObject(ctx, catObj)

	task := &model.Task{
		TaskID:                 "task_001",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_tsk",
		CommandCatalogObjectID: "cmd_cat",
		JSON:                   []byte(`{"cmd":"move"}`),
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}

	if err := taskStore.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	got, err := taskStore.GetTask(ctx, "task_001")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Status != model.TaskStatusPending {
		t.Fatalf("expected status 'pending', got '%s'", got.Status)
	}
}

func TestTaskStore_ListByStatus(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	entityStore := NewEntityStore(pool)
	objectStore := NewObjectStore(pool)
	taskStore := NewTaskStore(pool)
	ctx := context.Background()

	asset := &model.Entity{
		EntityID: "as_ls", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	entityStore.CreateEntity(ctx, asset)

	catObj := &model.Object{
		ObjectID: "cc_ls", Type: "command_catalog",
		OwnerType: model.OwnerTypeSystem, OwnerID: "system",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	objectStore.CreateObject(ctx, catObj)

	task1 := &model.Task{
		TaskID: "ts1", Status: model.TaskStatusPending, AssetID: "as_ls",
		CommandCatalogObjectID: "cc_ls", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	task2 := &model.Task{
		TaskID: "ts2", Status: model.TaskStatusCompleted, AssetID: "as_ls",
		CommandCatalogObjectID: "cc_ls", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	taskStore.CreateTask(ctx, task1)
	taskStore.CreateTask(ctx, task2)

	pending, _ := taskStore.ListTasks(ctx, store.WithTaskStatus(model.TaskStatusPending))
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending task, got %d", len(pending))
	}

	completed, _ := taskStore.ListTasks(ctx, store.WithTaskStatus(model.TaskStatusCompleted))
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed task, got %d", len(completed))
	}
}

func TestTaskStore_Upsert(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	entityStore := NewEntityStore(pool)
	objectStore := NewObjectStore(pool)
	taskStore := NewTaskStore(pool)
	ctx := context.Background()

	asset := &model.Entity{
		EntityID: "as_up", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	entityStore.CreateEntity(ctx, asset)

	catObj := &model.Object{
		ObjectID: "cc_up", Type: "command_catalog",
		OwnerType: model.OwnerTypeSystem, OwnerID: "system",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	objectStore.CreateObject(ctx, catObj)

	task := &model.Task{
		TaskID: "ts_up", Status: model.TaskStatusPending, AssetID: "as_up",
		CommandCatalogObjectID: "cc_up", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	taskStore.UpsertTask(ctx, task)

	task.Status = model.TaskStatusAcknowledged
	task.UpdatedAt = time.Now()
	taskStore.UpsertTask(ctx, task)

	got, _ := taskStore.GetTask(ctx, "ts_up")
	if got.Status != model.TaskStatusAcknowledged {
		t.Fatalf("expected 'acknowledged' after upsert, got '%s'", got.Status)
	}
}

func TestObservationStore_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	entityStore := NewEntityStore(pool)
	obsStore := NewObservationStore(pool)
	ctx := context.Background()

	source := &model.Entity{
		EntityID: "src_asset", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	entityStore.CreateEntity(ctx, source)

	obs := &model.Observation{
		ObservationID: "obs_001",
		SourceAssetID: "src_asset",
		JSON:          []byte(`{"lat":40.7,"lon":-74.0}`),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if err := obsStore.CreateObservation(ctx, obs); err != nil {
		t.Fatalf("CreateObservation failed: %v", err)
	}

	got, err := obsStore.GetObservation(ctx, "obs_001")
	if err != nil {
		t.Fatalf("GetObservation failed: %v", err)
	}
	if got.ObservationID != "obs_001" {
		t.Fatalf("expected 'obs_001', got '%s'", got.ObservationID)
	}
}

func TestObservationStore_ListBySourceAsset(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	entityStore := NewEntityStore(pool)
	obsStore := NewObservationStore(pool)
	ctx := context.Background()

	source := &model.Entity{
		EntityID: "src2", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	entityStore.CreateEntity(ctx, source)

	obs1 := &model.Observation{
		ObservationID: "ob1", SourceAssetID: "src2",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	obs2 := &model.Observation{
		ObservationID: "ob2", SourceAssetID: "src2",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	obsStore.CreateObservation(ctx, obs1)
	obsStore.CreateObservation(ctx, obs2)

	results, _ := obsStore.ListObservations(ctx, store.WithObservationSourceAssetID("src2"))
	if len(results) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(results))
	}
}

func TestObservationStore_Upsert(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	entityStore := NewEntityStore(pool)
	obsStore := NewObservationStore(pool)
	ctx := context.Background()

	source := &model.Entity{
		EntityID: "src_ups", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	entityStore.CreateEntity(ctx, source)

	obs := &model.Observation{
		ObservationID: "obs_ups", SourceAssetID: "src_ups",
		JSON: []byte(`{"v":1}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	obsStore.UpsertObservation(ctx, obs)

	obs.JSON = []byte(`{"v":2}`)
	obs.UpdatedAt = time.Now()
	obsStore.UpsertObservation(ctx, obs)

	got, _ := obsStore.GetObservation(ctx, "obs_ups")
	if string(got.JSON) != `{"v":2}` {
		t.Fatalf("expected '{\"v\":2}', got '%s'", string(got.JSON))
	}
}
