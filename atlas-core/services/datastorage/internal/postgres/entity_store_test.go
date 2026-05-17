package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

func TestEntityStore_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	entity := &model.Entity{
		EntityID:  "asset_001",
		Type:      model.EntityTypeAsset,
		JSON:      []byte(`{"name":"Test Asset"}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.CreateEntity(ctx, entity); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	got, err := s.GetEntity(ctx, "asset_001")
	if err != nil {
		t.Fatalf("GetEntity failed: %v", err)
	}
	if got.EntityID != "asset_001" {
		t.Fatalf("expected entity_id 'asset_001', got '%s'", got.EntityID)
	}
	if got.Type != model.EntityTypeAsset {
		t.Fatalf("expected type 'asset', got '%s'", got.Type)
	}
}

func TestEntityStore_NotFound(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	_, err := s.GetEntity(context.Background(), "nonexistent")
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEntityStore_Conflict(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	entity := &model.Entity{
		EntityID:  "dup_001",
		Type:      model.EntityTypeAsset,
		JSON:      []byte(`{}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.CreateEntity(ctx, entity); err != nil {
		t.Fatalf("first CreateEntity failed: %v", err)
	}
	if err := s.CreateEntity(ctx, entity); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate, got %v", err)
	}
}

func TestEntityStore_Update(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	entity := &model.Entity{
		EntityID:  "upd_001",
		Type:      model.EntityTypeAsset,
		JSON:      []byte(`{"v":1}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateEntity(ctx, entity); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	entity.JSON = []byte(`{"v":2}`)
	entity.UpdatedAt = time.Now().UTC()
	if err := s.UpdateEntity(ctx, entity); err != nil {
		t.Fatalf("UpdateEntity failed: %v", err)
	}

	got, err := s.GetEntity(ctx, "upd_001")
	if err != nil {
		t.Fatalf("GetEntity failed: %v", err)
	}
	assertJSONEqual(t, got.JSON, []byte(`{"v":2}`))
}

func TestEntityStore_UpdateNotFound(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	entity := &model.Entity{
		EntityID: "ghost",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`{}`),
	}

	err := s.UpdateEntity(context.Background(), entity)
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEntityStore_Delete(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	entity := &model.Entity{
		EntityID:  "del_001",
		Type:      model.EntityTypeTrack,
		JSON:      []byte(`{}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateEntity(ctx, entity); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	if err := s.DeleteEntity(ctx, "del_001"); err != nil {
		t.Fatalf("DeleteEntity failed: %v", err)
	}

	_, err := s.GetEntity(ctx, "del_001")
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestEntityStore_DeleteNotFound(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	err := s.DeleteEntity(context.Background(), "ghost")
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEntityStore_Upsert(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	entity := &model.Entity{
		EntityID:  "ups_001",
		Type:      model.EntityTypeAsset,
		JSON:      []byte(`{"v":1}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Insert via upsert
	if err := s.UpsertEntity(ctx, entity); err != nil {
		t.Fatalf("UpsertEntity insert failed: %v", err)
	}

	got, err := s.GetEntity(ctx, "ups_001")
	if err != nil {
		t.Fatalf("GetEntity failed: %v", err)
	}
	assertJSONEqual(t, got.JSON, []byte(`{"v":1}`))

	// Update via upsert
	entity.JSON = []byte(`{"v":2}`)
	entity.UpdatedAt = time.Now().UTC()
	if err := s.UpsertEntity(ctx, entity); err != nil {
		t.Fatalf("UpsertEntity update failed: %v", err)
	}

	got, err = s.GetEntity(ctx, "ups_001")
	if err != nil {
		t.Fatalf("GetEntity failed: %v", err)
	}
	assertJSONEqual(t, got.JSON, []byte(`{"v":2}`))
}

func TestEntityStore_UpdateAdvancesVersion(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	entity := &model.Entity{
		EntityID:  "ver_001",
		Type:      model.EntityTypeAsset,
		JSON:      []byte(`{}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateEntity(ctx, entity); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}
	if entity.Version != 1 {
		t.Fatalf("expected version 1 after create, got %d", entity.Version)
	}

	entity.UpdatedAt = time.Now().UTC()
	if err := s.UpdateEntity(ctx, entity); err != nil {
		t.Fatalf("UpdateEntity failed: %v", err)
	}
	if entity.Version != 2 {
		t.Fatalf("expected version 2 after update, got %d", entity.Version)
	}

	got, err := s.GetEntity(ctx, "ver_001")
	if err != nil {
		t.Fatalf("GetEntity failed: %v", err)
	}
	if got.Version != 2 {
		t.Fatalf("expected DB version 2, got %d", got.Version)
	}
}

func TestEntityStore_UpdateRejectsStaleVersion(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	entity := &model.Entity{
		EntityID:  "stale_001",
		Type:      model.EntityTypeAsset,
		JSON:      []byte(`{}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateEntity(ctx, entity); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	// First reader takes a snapshot of the entity at version 1.
	stale := *entity

	// Second writer advances the entity to version 2.
	entity.UpdatedAt = time.Now().UTC()
	if err := s.UpdateEntity(ctx, entity); err != nil {
		t.Fatalf("UpdateEntity (winner) failed: %v", err)
	}

	// First reader's update with the stale version is rejected.
	stale.UpdatedAt = time.Now().UTC()
	err := s.UpdateEntity(ctx, &stale)
	if !errors.Is(err, model.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

func TestEntityStore_ListByType(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	asset1 := &model.Entity{
		EntityID: "a1", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	track1 := &model.Entity{
		EntityID: "t1", Type: model.EntityTypeTrack,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	asset2 := &model.Entity{
		EntityID: "a2", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := s.CreateEntity(ctx, asset1); err != nil {
		t.Fatalf("CreateEntity asset1 failed: %v", err)
	}
	if err := s.CreateEntity(ctx, track1); err != nil {
		t.Fatalf("CreateEntity track1 failed: %v", err)
	}
	if err := s.CreateEntity(ctx, asset2); err != nil {
		t.Fatalf("CreateEntity asset2 failed: %v", err)
	}

	assetsRes, err := s.ListEntities(ctx, store.EntityListParams{Filters: []store.EntityFilter{store.WithEntityType(model.EntityTypeAsset)}})
	if err != nil {
		t.Fatalf("ListEntities failed: %v", err)
	}
	assets := assetsRes.Entities
	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}
}

func TestEntityStore_ListAll(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	results, err := s.ListEntities(ctx, store.EntityListParams{})
	if err != nil {
		t.Fatalf("ListEntities failed: %v", err)
	}
	if len(results.Entities) != 0 {
		t.Fatalf("expected 0 entities, got %d", len(results.Entities))
	}

	entity := &model.Entity{
		EntityID: "la_001", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.CreateEntity(ctx, entity); err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	results, err = s.ListEntities(ctx, store.EntityListParams{})
	if err != nil {
		t.Fatalf("ListEntities failed: %v", err)
	}
	if len(results.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(results.Entities))
	}
}
