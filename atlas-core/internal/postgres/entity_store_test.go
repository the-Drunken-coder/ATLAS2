package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/store"
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
	if err != model.ErrNotFound {
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
	if err := s.CreateEntity(ctx, entity); err != model.ErrConflict {
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
	s.CreateEntity(ctx, entity)

	entity.JSON = []byte(`{"v":2}`)
	entity.UpdatedAt = time.Now().UTC()
	if err := s.UpdateEntity(ctx, entity); err != nil {
		t.Fatalf("UpdateEntity failed: %v", err)
	}

	got, _ := s.GetEntity(ctx, "upd_001")
	if string(got.JSON) != `{"v":2}` {
		t.Fatalf("expected json '{\"v\":2}', got '%s'", string(got.JSON))
	}
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
	if err != model.ErrNotFound {
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
	s.CreateEntity(ctx, entity)

	if err := s.DeleteEntity(ctx, "del_001"); err != nil {
		t.Fatalf("DeleteEntity failed: %v", err)
	}

	_, err := s.GetEntity(ctx, "del_001")
	if err != model.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestEntityStore_DeleteNotFound(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	err := s.DeleteEntity(context.Background(), "ghost")
	if err != model.ErrNotFound {
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

	got, _ := s.GetEntity(ctx, "ups_001")
	if string(got.JSON) != `{"v":1}` {
		t.Fatalf("expected '{\"v\":1}', got '%s'", string(got.JSON))
	}

	// Update via upsert
	entity.JSON = []byte(`{"v":2}`)
	entity.UpdatedAt = time.Now().UTC()
	if err := s.UpsertEntity(ctx, entity); err != nil {
		t.Fatalf("UpsertEntity update failed: %v", err)
	}

	got, _ = s.GetEntity(ctx, "ups_001")
	if string(got.JSON) != `{"v":2}` {
		t.Fatalf("expected '{\"v\":2}', got '%s'", string(got.JSON))
	}
}

func TestEntityStore_ListByType(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	asset1 := &model.Entity{
		EntityID:  "a1", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	track1 := &model.Entity{
		EntityID:  "t1", Type: model.EntityTypeTrack,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	asset2 := &model.Entity{
		EntityID:  "a2", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	s.CreateEntity(ctx, asset1)
	s.CreateEntity(ctx, track1)
	s.CreateEntity(ctx, asset2)

	assets, err := s.ListEntities(ctx, store.WithEntityType(model.EntityTypeAsset))
	if err != nil {
		t.Fatalf("ListEntities failed: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}
}

func TestEntityStore_ListAll(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	s := NewEntityStore(pool)
	ctx := context.Background()

	results, err := s.ListEntities(ctx)
	if err != nil {
		t.Fatalf("ListEntities failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 entities, got %d", len(results))
	}

	entity := &model.Entity{
		EntityID:  "la_001", Type: model.EntityTypeAsset,
		JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	s.CreateEntity(ctx, entity)

	results, err = s.ListEntities(ctx)
	if err != nil {
		t.Fatalf("ListEntities failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(results))
	}
}
