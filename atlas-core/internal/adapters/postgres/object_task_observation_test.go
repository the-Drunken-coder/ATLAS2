package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
)

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed.UTC()
}

func TestObjectStore_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj := &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeDocument,
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
		ObjectID: "o1", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeEntity,
		OwnerID: "entity_a", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	obj2 := &model.Object{
		ObjectID: "o2", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeEntity,
		OwnerID: "entity_b", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := s.CreateObject(ctx, obj1); err != nil {
		t.Fatalf("CreateObject obj1 failed: %v", err)
	}
	if err := s.CreateObject(ctx, obj2); err != nil {
		t.Fatalf("CreateObject obj2 failed: %v", err)
	}

	results, err := s.ListObjects(ctx, ports.WithObjectOwnerType(model.OwnerTypeEntity), ports.WithObjectOwnerID("entity_a"))
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 object, got %d", len(results))
	}
	if results[0].ObjectID != "o1" {
		t.Fatalf("expected 'o1', got '%s'", results[0].ObjectID)
	}

	byOwnerIDOnly, err := s.ListObjects(ctx, ports.WithObjectOwnerID("entity_a"))
	if err != nil {
		t.Fatalf("ListObjects by owner_id failed: %v", err)
	}
	if len(byOwnerIDOnly) != 1 || byOwnerIDOnly[0].ObjectID != "o1" {
		t.Fatalf("expected owner_id-only filter to return o1, got %+v", byOwnerIDOnly)
	}
}

func TestObjectStore_UpdateAndDelete(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj := &model.Object{
		ObjectID: "od1", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeTask,
		OwnerID: "task_a", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.CreateObject(ctx, obj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

	obj.Type = model.ObjectTypePhoto
	obj.UpdatedAt = time.Now()
	if err := s.UpdateObject(ctx, obj); err != nil {
		t.Fatalf("UpdateObject failed: %v", err)
	}

	got, err := s.GetObject(ctx, "od1")
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	if got.Type != model.ObjectTypePhoto {
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
		ObjectID: "ups_obj", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem,
		OwnerID: "sys", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := s.UpsertObject(ctx, obj); err != nil {
		t.Fatalf("UpsertObject insert failed: %v", err)
	}

	obj.Type = model.ObjectTypePhoto
	obj.UpdatedAt = time.Now()
	if err := s.UpsertObject(ctx, obj); err != nil {
		t.Fatalf("UpsertObject update failed: %v", err)
	}

	got, err := s.GetObject(ctx, "ups_obj")
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	if got.Type != model.ObjectTypePhoto {
		t.Fatalf("expected 'photo' after upsert, got '%s'", got.Type)
	}
}

func TestObjectStore_ListByType(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj1 := &model.Object{
		ObjectID: "lt1", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem,
		OwnerID: "sys", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	obj2 := &model.Object{
		ObjectID: "lt2", Type: model.ObjectTypePhoto, OwnerType: model.OwnerTypeSystem,
		OwnerID: "sys", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := s.CreateObject(ctx, obj1); err != nil {
		t.Fatalf("CreateObject obj1 failed: %v", err)
	}
	if err := s.CreateObject(ctx, obj2); err != nil {
		t.Fatalf("CreateObject obj2 failed: %v", err)
	}

	photos, err := s.ListObjects(ctx, ports.WithObjectType(model.ObjectTypePhoto))
	if err != nil {
		t.Fatalf("ListObjects photos failed: %v", err)
	}
	if len(photos) != 1 {
		t.Fatalf("expected 1 photo, got %d", len(photos))
	}

	logs, err := s.ListObjects(ctx, ports.WithObjectType(model.ObjectTypeLog))
	if err != nil {
		t.Fatalf("ListObjects logs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
}

func TestObjectStore_UpdateAndGetManifest(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	objJSON, err := json.Marshal(map[string]any{
		"manifest": map[string]any{
			"files": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("Marshal JSON failed: %v", err)
	}

	obj := &model.Object{
		ObjectID:  "manifest_obj",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "sys",
		JSON:      objJSON,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateObject(ctx, obj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

	manifest := &model.ObjectManifest{
		Files: map[string]model.ObjectFileInfo{
			"data.txt": {Size: 4, UpdatedAt: mustParseTime(t, "2026-05-03T00:00:00Z")},
		},
	}
	if err := s.UpdateObjectManifest(ctx, obj.ObjectID, manifest); err != nil {
		t.Fatalf("UpdateObjectManifest failed: %v", err)
	}

	got, err := s.GetObjectManifest(ctx, obj.ObjectID)
	if err != nil {
		t.Fatalf("GetObjectManifest failed: %v", err)
	}
	if got.Files["data.txt"].Size != 4 {
		t.Fatalf("expected manifest file size 4, got %d", got.Files["data.txt"].Size)
	}
}

func TestObjectStore_UpdateObjectPreservesManifestCache(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj := &model.Object{
		ObjectID:  "manifest_update_obj",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "sys",
		JSON:      []byte(`{"desc":"before"}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateObject(ctx, obj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

	manifest := &model.ObjectManifest{
		Files: map[string]model.ObjectFileInfo{
			"data.txt": {Size: 4, UpdatedAt: mustParseTime(t, "2026-05-03T00:00:00Z")},
		},
	}
	manifest = model.NormalizeManifest(manifest)
	if err := s.UpdateObjectManifest(ctx, obj.ObjectID, manifest); err != nil {
		t.Fatalf("UpdateObjectManifest failed: %v", err)
	}

	obj.JSON = []byte(`{"desc":"after"}`)
	obj.UpdatedAt = time.Now().UTC()
	if err := s.UpdateObject(ctx, obj); err != nil {
		t.Fatalf("UpdateObject failed: %v", err)
	}

	got, err := s.GetObject(ctx, obj.ObjectID)
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	assertJSONEqual(t, got.JSON, []byte(`{
		"desc": "after",
		"manifest": {
			"version": "`+manifest.Version+`",
			"files": {
				"data.txt": {
					"size": 4,
					"updated_at": "2026-05-03T00:00:00Z"
				}
			}
		},
		"manifest_version": "`+manifest.Version+`"
	}`))
}

func TestObjectStore_UpsertObjectPreservesManifestCacheOnConflict(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewObjectStore(pool)
	ctx := context.Background()

	obj := &model.Object{
		ObjectID:  "manifest_upsert_obj",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "sys",
		JSON:      []byte(`{"desc":"before"}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.UpsertObject(ctx, obj); err != nil {
		t.Fatalf("UpsertObject insert failed: %v", err)
	}

	manifest := &model.ObjectManifest{
		Files: map[string]model.ObjectFileInfo{
			"data.txt": {Size: 8, UpdatedAt: mustParseTime(t, "2026-05-04T00:00:00Z")},
		},
	}
	manifest = model.NormalizeManifest(manifest)
	if err := s.UpdateObjectManifest(ctx, obj.ObjectID, manifest); err != nil {
		t.Fatalf("UpdateObjectManifest failed: %v", err)
	}

	obj.JSON = []byte(`{"desc":"after"}`)
	obj.UpdatedAt = time.Now().UTC()
	if err := s.UpsertObject(ctx, obj); err != nil {
		t.Fatalf("UpsertObject conflict update failed: %v", err)
	}

	got, err := s.GetObjectManifest(ctx, obj.ObjectID)
	if err != nil {
		t.Fatalf("GetObjectManifest failed: %v", err)
	}
	if got.Version != manifest.Version {
		t.Fatalf("expected manifest version %q, got %q", manifest.Version, got.Version)
	}
	if got.Files["data.txt"].Size != 8 {
		t.Fatalf("expected preserved manifest file size 8, got %d", got.Files["data.txt"].Size)
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
	if err := entityStore.CreateEntity(ctx, asset); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	catObj := &model.Object{
		ObjectID: "cmd_cat", Type: model.ObjectTypeDocument,
		OwnerType: model.OwnerTypeSystem, OwnerID: "system",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := objectStore.CreateObject(ctx, catObj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

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
	if err := entityStore.CreateEntity(ctx, asset); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	catObj := &model.Object{
		ObjectID: "cc_ls", Type: model.ObjectTypeDocument,
		OwnerType: model.OwnerTypeSystem, OwnerID: "system",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := objectStore.CreateObject(ctx, catObj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

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

	if err := taskStore.CreateTask(ctx, task1); err != nil {
		t.Fatalf("CreateTask task1 failed: %v", err)
	}
	if err := taskStore.CreateTask(ctx, task2); err != nil {
		t.Fatalf("CreateTask task2 failed: %v", err)
	}

	pending, err := taskStore.ListTasks(ctx, ports.WithTaskStatus(model.TaskStatusPending))
	if err != nil {
		t.Fatalf("ListTasks pending failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending task, got %d", len(pending))
	}

	completed, err := taskStore.ListTasks(ctx, ports.WithTaskStatus(model.TaskStatusCompleted))
	if err != nil {
		t.Fatalf("ListTasks completed failed: %v", err)
	}
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
	if err := entityStore.CreateEntity(ctx, asset); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	catObj := &model.Object{
		ObjectID: "cc_up", Type: model.ObjectTypeDocument,
		OwnerType: model.OwnerTypeSystem, OwnerID: "system",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := objectStore.CreateObject(ctx, catObj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

	task := &model.Task{
		TaskID: "ts_up", Status: model.TaskStatusPending, AssetID: "as_up",
		CommandCatalogObjectID: "cc_up", JSON: []byte(`{}`),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := taskStore.UpsertTask(ctx, task); err != nil {
		t.Fatalf("UpsertTask insert failed: %v", err)
	}

	task.Status = model.TaskStatusAcknowledged
	task.UpdatedAt = time.Now()
	if err := taskStore.UpsertTask(ctx, task); err != nil {
		t.Fatalf("UpsertTask update failed: %v", err)
	}

	got, err := taskStore.GetTask(ctx, "ts_up")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Status != model.TaskStatusAcknowledged {
		t.Fatalf("expected 'acknowledged' after upsert, got '%s'", got.Status)
	}
}

func TestTaskStore_UpdateVersioningAndClassification(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	entityStore := NewEntityStore(pool)
	objectStore := NewObjectStore(pool)
	taskStore := NewTaskStore(pool)
	ctx := context.Background()

	asset := &model.Entity{
		EntityID: "asset_update_task", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := entityStore.CreateEntity(ctx, asset); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	catObj := &model.Object{
		ObjectID: "cmd_update_task", Type: model.ObjectTypeDocument,
		OwnerType: model.OwnerTypeSystem, OwnerID: "system",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := objectStore.CreateObject(ctx, catObj); err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

	task := &model.Task{
		TaskID:                 "task_update",
		Status:                 model.TaskStatusPending,
		AssetID:                "asset_update_task",
		CommandCatalogObjectID: "cmd_update_task",
		JSON:                   []byte(`{"step":1}`),
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}
	if err := taskStore.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	task.Status = model.TaskStatusAcknowledged
	task.UpdatedAt = time.Now().UTC()
	if err := taskStore.UpdateTask(ctx, task); err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}
	if task.Version != 2 {
		t.Fatalf("expected updated task version 2, got %d", task.Version)
	}

	stale := *task
	stale.Version = 1
	stale.UpdatedAt = time.Now().UTC()
	if err := taskStore.UpdateTask(ctx, &stale); !errors.Is(err, model.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict for stale task version, got %v", err)
	}

	missing := *task
	missing.TaskID = "task_missing"
	missing.Version = 1
	missing.UpdatedAt = time.Now().UTC()
	if err := taskStore.UpdateTask(ctx, &missing); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing task update, got %v", err)
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
	if err := entityStore.CreateEntity(ctx, source); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

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
	if err := entityStore.CreateEntity(ctx, source); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	obs1 := &model.Observation{
		ObservationID: "ob1", SourceAssetID: "src2",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	obs2 := &model.Observation{
		ObservationID: "ob2", SourceAssetID: "src2",
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := obsStore.CreateObservation(ctx, obs1); err != nil {
		t.Fatalf("CreateObservation obs1 failed: %v", err)
	}
	if err := obsStore.CreateObservation(ctx, obs2); err != nil {
		t.Fatalf("CreateObservation obs2 failed: %v", err)
	}

	results, err := obsStore.ListObservations(ctx, ports.WithObservationSourceAssetID("src2"))
	if err != nil {
		t.Fatalf("ListObservations failed: %v", err)
	}
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
	if err := entityStore.CreateEntity(ctx, source); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	obs := &model.Observation{
		ObservationID: "obs_ups", SourceAssetID: "src_ups",
		JSON: []byte(`{"v":1}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := obsStore.UpsertObservation(ctx, obs); err != nil {
		t.Fatalf("UpsertObservation insert failed: %v", err)
	}

	obs.JSON = []byte(`{"v":2}`)
	obs.UpdatedAt = time.Now()
	if err := obsStore.UpsertObservation(ctx, obs); err != nil {
		t.Fatalf("UpsertObservation update failed: %v", err)
	}

	got, err := obsStore.GetObservation(ctx, "obs_ups")
	if err != nil {
		t.Fatalf("GetObservation failed: %v", err)
	}
	assertJSONEqual(t, got.JSON, []byte(`{"v":2}`))
}

func TestObservationStore_UpdateVersioningAndClassification(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	entityStore := NewEntityStore(pool)
	obsStore := NewObservationStore(pool)
	ctx := context.Background()

	source := &model.Entity{
		EntityID: "src_update_obs", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := entityStore.CreateEntity(ctx, source); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	obs := &model.Observation{
		ObservationID: "obs_update",
		SourceAssetID: "src_update_obs",
		JSON:          []byte(`{"v":1}`),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := obsStore.CreateObservation(ctx, obs); err != nil {
		t.Fatalf("CreateObservation failed: %v", err)
	}

	obs.JSON = []byte(`{"v":2}`)
	obs.UpdatedAt = time.Now().UTC()
	if err := obsStore.UpdateObservation(ctx, obs); err != nil {
		t.Fatalf("UpdateObservation failed: %v", err)
	}
	if obs.Version != 2 {
		t.Fatalf("expected updated observation version 2, got %d", obs.Version)
	}

	stale := *obs
	stale.Version = 1
	stale.UpdatedAt = time.Now().UTC()
	if err := obsStore.UpdateObservation(ctx, &stale); !errors.Is(err, model.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict for stale observation version, got %v", err)
	}

	missing := *obs
	missing.ObservationID = "obs_missing"
	missing.Version = 1
	missing.UpdatedAt = time.Now().UTC()
	if err := obsStore.UpdateObservation(ctx, &missing); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing observation update, got %v", err)
	}
}
