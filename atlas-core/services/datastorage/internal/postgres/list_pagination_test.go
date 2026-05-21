package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

func TestStoreListPagination(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	t.Run("entity", func(t *testing.T) {
		s := NewEntityStore(pool)
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		for i, id := range []string{"pg_e1", "pg_e2", "pg_e3"} {
			e := &model.Entity{
				EntityID:  id,
				Type:      model.EntityTypeAsset,
				JSON:      []byte(`{}`),
				CreatedAt: base.Add(time.Duration(i) * time.Hour),
				UpdatedAt: base.Add(time.Duration(i) * time.Hour),
			}
			if err := s.CreateEntity(ctx, e); err != nil {
				t.Fatalf("CreateEntity %s: %v", id, err)
			}
		}

		page1, err := s.ListEntities(ctx, store.EntityListParams{PageSize: 2})
		if err != nil {
			t.Fatalf("page1: %v", err)
		}
		if len(page1.Entities) != 2 {
			t.Fatalf("page1 len: %d", len(page1.Entities))
		}
		if page1.Entities[0].EntityID != "pg_e3" || page1.Entities[1].EntityID != "pg_e2" {
			t.Fatalf("page1 order: %#v", page1.Entities)
		}
		if page1.NextPageToken == "" {
			t.Fatal("expected next_page_token")
		}

		page2, err := s.ListEntities(ctx, store.EntityListParams{PageSize: 2, PageToken: page1.NextPageToken})
		if err != nil {
			t.Fatalf("page2: %v", err)
		}
		if len(page2.Entities) != 1 || page2.Entities[0].EntityID != "pg_e1" {
			t.Fatalf("page2: %#v", page2.Entities)
		}
		if page2.NextPageToken != "" {
			t.Fatalf("expected empty next token, got %q", page2.NextPageToken)
		}

		filteredPage1, err := s.ListEntities(ctx, store.EntityListParams{
			Filters:  []store.EntityFilter{store.WithEntityType(model.EntityTypeAsset)},
			PageSize: 2,
		})
		if err != nil {
			t.Fatalf("filtered page1: %v", err)
		}
		if len(filteredPage1.Entities) != 2 {
			t.Fatalf("filtered page1 expected 2 rows, got %d", len(filteredPage1.Entities))
		}

		filteredPage2, err := s.ListEntities(ctx, store.EntityListParams{
			Filters:   []store.EntityFilter{store.WithEntityType(model.EntityTypeAsset)},
			PageSize:  2,
			PageToken: filteredPage1.NextPageToken,
		})
		if err != nil {
			t.Fatalf("filtered page2: %v", err)
		}
		if len(filteredPage2.Entities) != 1 {
			t.Fatalf("filtered page2 expected 1 row, got %d", len(filteredPage2.Entities))
		}

		if _, err := s.ListEntities(ctx, store.EntityListParams{PageToken: "not-a-token"}); err == nil {
			t.Fatal("expected error for bad token")
		}
		if _, err := s.ListEntities(ctx, store.EntityListParams{PageSize: -1}); err == nil {
			t.Fatal("expected error for negative page_size")
		}
		if _, err := s.ListEntities(ctx, store.EntityListParams{PageSize: 501}); err == nil {
			t.Fatal("expected error for page_size > 500")
		}
	})

	t.Run("object", func(t *testing.T) {
		s := NewObjectStore(pool)
		base := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
		for i, id := range []string{"pg_o1", "pg_o2", "pg_o3"} {
			o := &model.Object{
				ObjectID:  id,
				Type:      model.ObjectTypeLog,
				OwnerType: model.OwnerTypeSystem,
				OwnerID:   "sys",
				JSON:      []byte(`{}`),
				CreatedAt: base.Add(time.Duration(i) * time.Minute),
				UpdatedAt: base.Add(time.Duration(i) * time.Minute),
			}
			if err := s.CreateObject(ctx, o); err != nil {
				t.Fatalf("CreateObject %s: %v", id, err)
			}
		}

		p1, err := s.ListObjects(ctx, store.ObjectListParams{PageSize: 2})
		if err != nil {
			t.Fatalf("page1: %v", err)
		}
		if len(p1.Objects) != 2 || p1.Objects[0].ObjectID != "pg_o3" {
			t.Fatalf("unexpected page1: %#v", p1.Objects)
		}
		p2, err := s.ListObjects(ctx, store.ObjectListParams{PageSize: 2, PageToken: p1.NextPageToken})
		if err != nil {
			t.Fatalf("page2: %v", err)
		}
		if len(p2.Objects) != 1 || p2.NextPageToken != "" {
			t.Fatalf("unexpected page2: %#v next=%q", p2.Objects, p2.NextPageToken)
		}

		if _, err := s.ListObjects(ctx, store.ObjectListParams{PageToken: "not-a-token"}); err == nil {
			t.Fatal("expected error for bad token")
		}
	})

	t.Run("task", func(t *testing.T) {
		es := NewEntityStore(pool)
		os := NewObjectStore(pool)
		ts := NewTaskStore(pool)

		asset := &model.Entity{EntityID: "pg_asset", Type: model.EntityTypeAsset, JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := es.CreateEntity(ctx, asset); err != nil {
			t.Fatalf("entity: %v", err)
		}
		cc := &model.Object{ObjectID: "pg_cc", Type: model.ObjectTypeCommandCatalog, OwnerType: model.OwnerTypeSystem, OwnerID: "system", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := os.CreateObject(ctx, cc); err != nil {
			t.Fatalf("catalog: %v", err)
		}

		base := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
		for i, tid := range []string{"pg_t1", "pg_t2", "pg_t3"} {
			task := &model.Task{
				TaskID:                 tid,
				Status:                 model.TaskStatusPending,
				AssetID:                "pg_asset",
				CommandCatalogObjectID: "pg_cc",
				JSON:                   []byte(`{"cmd":"noop"}`),
				CreatedAt:              base.Add(time.Duration(i) * time.Second),
				UpdatedAt:              base.Add(time.Duration(i) * time.Second),
			}
			if err := ts.CreateTask(ctx, task); err != nil {
				t.Fatalf("task %s: %v", tid, err)
			}
		}

		p1, err := ts.ListTasks(ctx, store.TaskListParams{PageSize: 2})
		if err != nil {
			t.Fatalf("page1: %v", err)
		}
		if len(p1.Tasks) != 2 || p1.Tasks[0].TaskID != "pg_t3" {
			t.Fatalf("bad page1: %#v", p1.Tasks)
		}
		p2, err := ts.ListTasks(ctx, store.TaskListParams{PageSize: 2, PageToken: p1.NextPageToken})
		if err != nil || len(p2.Tasks) != 1 || p2.NextPageToken != "" {
			t.Fatalf("bad page2: %#v err=%v", p2.Tasks, err)
		}

		if _, err := ts.ListTasks(ctx, store.TaskListParams{PageToken: "not-a-token"}); err == nil {
			t.Fatal("expected error for bad token")
		}
	})

	t.Run("observation", func(t *testing.T) {
		es := NewEntityStore(pool)
		obs := NewObservationStore(pool)

		src := &model.Entity{EntityID: "pg_src", Type: model.EntityTypeAsset, JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := es.CreateEntity(ctx, src); err != nil {
			t.Fatalf("entity: %v", err)
		}

		base := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
		for i, oid := range []string{"pg_ob1", "pg_ob2", "pg_ob3"} {
			o := &model.Observation{
				ObservationID: oid,
				SourceAssetID: "pg_src",
				StartedAt:     base,
				JSON:          []byte(`{"extra":{}}`),
				CreatedAt:     base.Add(time.Duration(i) * time.Millisecond),
				UpdatedAt:     base.Add(time.Duration(i) * time.Millisecond),
			}
			if err := obs.CreateObservation(ctx, o); err != nil {
				t.Fatalf("obs %s: %v", oid, err)
			}
		}

		p1, err := obs.ListObservations(ctx, store.ObservationListParams{PageSize: 2})
		if err != nil {
			t.Fatalf("page1: %v", err)
		}
		if len(p1.Observations) != 2 || p1.Observations[0].ObservationID != "pg_ob3" {
			t.Fatalf("bad page1: %#v", p1.Observations)
		}
		p2, err := obs.ListObservations(ctx, store.ObservationListParams{PageSize: 2, PageToken: p1.NextPageToken})
		if err != nil || len(p2.Observations) != 1 || p2.NextPageToken != "" {
			t.Fatalf("bad page2: %#v err=%v", p2.Observations, err)
		}

		if _, err := obs.ListObservations(ctx, store.ObservationListParams{PageToken: "not-a-token"}); err == nil {
			t.Fatal("expected error for bad token")
		}
	})
}
