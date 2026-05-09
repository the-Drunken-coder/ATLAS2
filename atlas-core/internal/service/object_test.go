package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
)

func TestObjectFunctions_ValidateRequiredFields(t *testing.T) {
	f := ObjectFunctions{}
	obj := &model.Object{ObjectID: "obj_001", OwnerType: model.OwnerTypeSystem, OwnerID: "system", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObject(nil, obj); err == nil {
		t.Fatal("expected error for empty type")
	}
	obj.Type = model.ObjectTypeLog
	obj.OwnerType = ""
	if err := f.CreateObject(nil, obj); err == nil {
		t.Fatal("expected error for empty owner_type")
	}
	obj.OwnerType = model.OwnerTypeSystem
	obj.OwnerID = ""
	if err := f.CreateObject(nil, obj); err == nil {
		t.Fatal("expected error for empty owner_id")
	}
}

func TestObjectFunctions_RejectUnsafeObjectID(t *testing.T) {
	f := ObjectFunctions{}
	obj := &model.Object{ObjectID: "../obj", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
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

func TestObjectFunctions_CreateObjectRollsBackMetadataOnStorageFailure(t *testing.T) {
	deleted := false
	pg := &fakeObjectStore{
		createFn: func(context.Context, *model.Object) error { return nil },
		deleteFn: func(context.Context, string) error { deleted = true; return nil },
	}
	storage := fakeObjectStorage{createFolderFn: func(string) error { return fmt.Errorf("boom") }}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	obj := &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}
	if err := f.CreateObject(context.Background(), obj); err == nil {
		t.Fatal("expected create object failure")
	}
	if !deleted {
		t.Fatal("expected metadata rollback to run")
	}
}

func TestObjectFunctions_CreateObjectReportsRollbackFailure(t *testing.T) {
	pg := &fakeObjectStore{
		createFn: func(context.Context, *model.Object) error { return nil },
		deleteFn: func(context.Context, string) error { return fmt.Errorf("delete failed") },
	}
	storage := fakeObjectStorage{
		createFolderFn: func(string) error { return fmt.Errorf("boom") },
		deleteFolderFn: func(string) error { return fmt.Errorf("cleanup failed") },
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	obj := &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}
	err := f.CreateObject(context.Background(), obj)
	if err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("expected rollback failure details, got %v", err)
	}
}

func TestObjectFunctions_CreateObjectDoesNotFailOnManifestCacheRefreshFailure(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}))
	deleted := false
	pg := &fakeObjectStore{
		createFn:      func(context.Context, *model.Object) error { return nil },
		deleteFn:      func(context.Context, string) error { deleted = true; return nil },
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) { return nil, model.ErrNotFound },
		updateManifestFn: func(context.Context, string, *model.ObjectManifest, ...time.Time) error {
			return fmt.Errorf("cache unavailable")
		},
	}
	storage := fakeObjectStorage{
		createFolderFn: func(string) error { return nil },
		readManifestFn: func(string) ([]byte, error) { return manifestData, nil },
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())

	if err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}); err != nil {
		t.Fatalf("expected create to succeed despite manifest cache refresh failure, got %v", err)
	}
	if deleted {
		t.Fatal("did not expect rollback after durable object creation")
	}
}

func TestObjectFunctions_DeleteObjectRestoresMetadataOnStorageFailure(t *testing.T) {
	upserted := false
	pg := &fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}, nil
		},
		deleteFn: func(context.Context, string) error { return nil },
		upsertFn: func(context.Context, *model.Object) error { upserted = true; return nil },
	}
	storage := fakeObjectStorage{deleteFolderFn: func(string) error { return fmt.Errorf("storage delete failed") }}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	if err := f.DeleteObject(context.Background(), "obj_001"); err == nil {
		t.Fatal("expected delete failure")
	}
	if !upserted {
		t.Fatal("expected metadata restore on storage failure")
	}
}

func TestObjectFunctions_UpsertObjectDoesNotFailOnManifestCacheRefreshFailure(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}))
	deleted := false
	pg := &fakeObjectStore{
		getFn:         func(context.Context, string) (*model.Object, error) { return nil, model.ErrNotFound },
		upsertFn:      func(context.Context, *model.Object) error { return nil },
		deleteFn:      func(context.Context, string) error { deleted = true; return nil },
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) { return nil, model.ErrNotFound },
		updateManifestFn: func(context.Context, string, *model.ObjectManifest, ...time.Time) error {
			return fmt.Errorf("cache unavailable")
		},
	}
	storage := fakeObjectStorage{
		existsFn:       func(string) (bool, error) { return false, nil },
		createFolderFn: func(string) error { return nil },
		readManifestFn: func(string) ([]byte, error) { return manifestData, nil },
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())

	if err := f.UpsertObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}); err != nil {
		t.Fatalf("expected upsert to succeed despite manifest cache refresh failure, got %v", err)
	}
	if deleted {
		t.Fatal("did not expect rollback after durable object upsert")
	}
}

func TestObjectFunctions_CreateObjectRecoversPendingIdempotencyClaim(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}))
	created := false
	completed := false
	pg := &fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			if created {
				return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}, nil
			}
			return nil, model.ErrNotFound
		},
		createFn: func(context.Context, *model.Object) error {
			created = true
			return nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return nil, model.ErrNotFound
		},
	}
	storage := fakeObjectStorage{
		createFolderFn: func(string) error { return nil },
		readManifestFn: func(string) ([]byte, error) { return manifestData, nil },
	}
	idem := fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (ports.IdempotencyRecord, bool, error) {
			return ports.IdempotencyRecord{ResourceID: "obj_001", Status: ports.IdempotencyStatusPending}, false, nil
		},
		markCompletedFn: func(context.Context, string, string) error {
			completed = true
			return nil
		},
	}
	f := NewObjectFunctions(pg, storage, idem, testLogger())

	if err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("client-1")); err != nil {
		t.Fatalf("expected pending claim recovery to succeed, got %v", err)
	}
	if !created {
		t.Fatal("expected object creation to resume for pending claim")
	}
	if !completed {
		t.Fatal("expected idempotency key to be marked completed")
	}
}

func TestObjectFunctions_CreateObjectWithFreshIdempotencyKeyStillConflictsOnDuplicateID(t *testing.T) {
	markedFailed := false
	pg := &fakeObjectStore{
		createFn: func(context.Context, *model.Object) error { return model.ErrConflict },
	}
	f := NewObjectFunctions(pg, fakeObjectStorage{}, fakeIdempotencyStore{
		tryBeginFn: func(context.Context, string, string, string) (ports.IdempotencyRecord, bool, error) {
			return ports.IdempotencyRecord{ResourceID: "obj_001", Status: ports.IdempotencyStatusPending}, true, nil
		},
		markFailedFn: func(context.Context, string, string) error {
			markedFailed = true
			return nil
		},
	}, testLogger())

	err := f.CreateObject(context.Background(), &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
	}, WithIdempotencyKey("fresh-key"))
	if !errors.Is(err, model.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if !markedFailed {
		t.Fatal("expected fresh idempotency claim to be marked failed on duplicate object")
	}
}
