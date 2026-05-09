package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
)

func TestObjectFunctions_ReconcileRepairsDrift(t *testing.T) {
	manifestData, _ := json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{"a.txt": {Size: 1, UpdatedAt: time.Now().UTC()}}}))
	deletedFolder := ""
	createdFolder := ""
	restoredObjectID := ""
	pg := &fakeObjectStore{
		createFn: func(_ context.Context, obj *model.Object) error {
			restoredObjectID = obj.ObjectID
			if obj.Type != model.ObjectTypeLog || obj.OwnerType != model.OwnerTypeSystem || obj.OwnerID != "system" {
				t.Fatalf("unexpected restored object metadata: %+v", obj)
			}
			return nil
		},
		listFn: func(context.Context, ...ports.ObjectFilter) ([]model.Object, error) {
			return []model.Object{{ObjectID: "db_only", Type: model.ObjectTypeLog}, {ObjectID: "shared", Type: model.ObjectTypeLog}}, nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}), nil
		},
	}
	storage := fakeObjectStorage{
		listFoldersFn:  func() ([]string, error) { return []string{"shared", "fs_only", "no_manifest"}, nil },
		deleteFolderFn: func(objectID string) error { deletedFolder = objectID; return nil },
		createFolderFn: func(objectID string) error { createdFolder = objectID; return nil },
		readManifestFn: func(objectID string) ([]byte, error) {
			if objectID == "no_manifest" {
				return nil, model.ErrNotFound
			}
			return manifestData, nil
		},
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	if err := f.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if restoredObjectID != "fs_only" {
		t.Fatalf("expected fs_only metadata restoration, got %q", restoredObjectID)
	}
	if deletedFolder != "no_manifest" {
		t.Fatalf("expected no_manifest folder deletion, got %q", deletedFolder)
	}
	if createdFolder != "db_only" {
		t.Fatalf("expected db_only folder recreation, got %q", createdFolder)
	}
	if pg.updatedManifestCalls == 0 {
		t.Fatal("expected manifest cache sync during reconciliation")
	}
}

func TestObjectFunctions_ReconcileRepairsMissingManifest(t *testing.T) {
	var repairedManifest []byte
	repaired := false
	pg := &fakeObjectStore{
		listFn: func(context.Context, ...ports.ObjectFilter) ([]model.Object, error) {
			return []model.Object{{ObjectID: "shared", Type: model.ObjectTypeLog}}, nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return nil, model.ErrNotFound
		},
	}
	storage := fakeObjectStorage{
		listFoldersFn: func() ([]string, error) { return []string{"shared"}, nil },
		listFilesFn:   func(string) ([]string, error) { return []string{"data.txt"}, nil },
		fileInfoFn: func(string, string) (model.ObjectFileInfo, error) {
			return model.ObjectFileInfo{Size: 4, UpdatedAt: mustParseTime(t, "2026-05-05T00:00:00Z")}, nil
		},
		readManifestFn: func(string) ([]byte, error) {
			if !repaired {
				return nil, model.ErrNotFound
			}
			return repairedManifest, nil
		},
		writeManifestFn: func(_ string, data []byte) error {
			repaired = true
			repairedManifest = append([]byte(nil), data...)
			return nil
		},
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	if err := f.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !repaired {
		t.Fatal("expected reconcile to repair missing manifest")
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(repairedManifest, &manifest); err != nil {
		t.Fatalf("expected repaired manifest JSON, got %v", err)
	}
	if manifest.Files["data.txt"].Size != 4 {
		t.Fatalf("expected repaired manifest to retain file index, got %+v", manifest.Files)
	}
}

func TestObjectFunctions_ReconcileRepairsMalformedManifestWithoutErasingFiles(t *testing.T) {
	var repairedManifest []byte
	pg := &fakeObjectStore{
		listFn: func(context.Context, ...ports.ObjectFilter) ([]model.Object, error) {
			return []model.Object{{ObjectID: "shared", Type: model.ObjectTypeLog}}, nil
		},
		getManifestFn: func(context.Context, string) (*model.ObjectManifest, error) {
			return nil, model.ErrNotFound
		},
	}
	storage := fakeObjectStorage{
		listFoldersFn: func() ([]string, error) { return []string{"shared"}, nil },
		listFilesFn:   func(string) ([]string, error) { return []string{"data.txt"}, nil },
		fileInfoFn: func(string, string) (model.ObjectFileInfo, error) {
			return model.ObjectFileInfo{Size: 7, UpdatedAt: mustParseTime(t, "2026-05-06T00:00:00Z")}, nil
		},
		readManifestFn: func(string) ([]byte, error) {
			if repairedManifest != nil {
				return repairedManifest, nil
			}
			return []byte(`{"files":`), nil
		},
		writeManifestFn: func(_ string, data []byte) error {
			repairedManifest = append([]byte(nil), data...)
			return nil
		},
	}
	f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
	if err := f.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(repairedManifest, &manifest); err != nil {
		t.Fatalf("expected repaired manifest JSON, got %v", err)
	}
	if manifest.Files["data.txt"].Size != 7 {
		t.Fatalf("expected repaired manifest to retain malformed-manifest file index, got %+v", manifest.Files)
	}
}
