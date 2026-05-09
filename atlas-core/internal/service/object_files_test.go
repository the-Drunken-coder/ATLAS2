package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
)

func TestObjectFunctions_ReadFileRequiresObjectRow(t *testing.T) {
	readCalled := false
	f := NewObjectFunctions(&fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			return nil, model.ErrNotFound
		},
	}, fakeObjectStorage{
		readFileFn: func(string, string) ([]byte, error) {
			readCalled = true
			return []byte("secret"), nil
		},
	}, fakeIdempotencyStore{}, testLogger())

	_, err := f.ReadFile(context.Background(), "obj_001", "data.txt")
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if readCalled {
		t.Fatal("expected missing object row to prevent file read")
	}
}

func TestObjectFunctions_ListFilesRequiresObjectRow(t *testing.T) {
	listCalled := false
	f := NewObjectFunctions(&fakeObjectStore{
		getFn: func(context.Context, string) (*model.Object, error) {
			return nil, model.ErrNotFound
		},
	}, fakeObjectStorage{
		listFilesFn: func(string) ([]string, error) {
			listCalled = true
			return []string{"data.txt"}, nil
		},
	}, fakeIdempotencyStore{}, testLogger())

	_, err := f.ListFiles(context.Background(), "obj_001")
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if listCalled {
		t.Fatal("expected missing object row to prevent file listing")
	}
}

func TestObjectFunctions_FileMutationsRebuildAndSyncManifest(t *testing.T) {
	cases := []struct {
		name          string
		mutate        func(ObjectFunctions) error
		expectedFiles map[string]int64
	}{
		{
			name: "write",
			mutate: func(f ObjectFunctions) error {
				return f.WriteFile(context.Background(), "obj_001", "data.txt", []byte("data"))
			},
			expectedFiles: map[string]int64{"data.txt": 4},
		},
		{
			name: "append",
			mutate: func(f ObjectFunctions) error {
				return f.AppendFile(context.Background(), "obj_001", "data.txt", []byte("more"))
			},
			expectedFiles: map[string]int64{"data.txt": 8},
		},
		{
			name: "delete",
			mutate: func(f ObjectFunctions) error {
				return f.DeleteFile(context.Background(), "obj_001", "data.txt")
			},
			expectedFiles: map[string]int64{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var manifestFile model.ObjectManifest
			var cachedManifest *model.ObjectManifest
			pg := &fakeObjectStore{
				getFn: func(context.Context, string) (*model.Object, error) {
					return &model.Object{ObjectID: "obj_001", Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system"}, nil
				},
				updateManifestFn: func(_ context.Context, _ string, manifest *model.ObjectManifest, _ ...time.Time) error {
					cachedManifest = model.NormalizeManifest(manifest)
					return nil
				},
			}
			storage := fakeObjectStorage{
				writeFileFn:  func(string, string, []byte) error { return nil },
				appendFileFn: func(string, string, []byte) error { return nil },
				deleteFileFn: func(string, string) error { return nil },
				listFilesFn: func(string) ([]string, error) {
					names := make([]string, 0, len(tc.expectedFiles))
					for name := range tc.expectedFiles {
						names = append(names, name)
					}
					sort.Strings(names)
					return names, nil
				},
				fileInfoFn: func(_ string, filename string) (model.ObjectFileInfo, error) {
					return model.ObjectFileInfo{
						Size:      tc.expectedFiles[filename],
						UpdatedAt: mustParseTime(t, "2026-05-05T00:00:00Z"),
					}, nil
				},
				writeManifestFn: func(_ string, data []byte) error {
					if err := json.Unmarshal(data, &manifestFile); err != nil {
						t.Fatalf("manifest write was not valid JSON: %v", err)
					}
					return nil
				},
			}
			f := NewObjectFunctions(pg, storage, fakeIdempotencyStore{}, testLogger())
			if err := tc.mutate(f); err != nil {
				t.Fatalf("%s failed: %v", tc.name, err)
			}
			if cachedManifest == nil {
				t.Fatal("expected database manifest cache update")
			}
			if len(manifestFile.Files) != len(tc.expectedFiles) {
				t.Fatalf("expected filesystem manifest files %+v, got %+v", tc.expectedFiles, manifestFile.Files)
			}
			if len(cachedManifest.Files) != len(tc.expectedFiles) {
				t.Fatalf("expected cached manifest files %+v, got %+v", tc.expectedFiles, cachedManifest.Files)
			}
			for name, size := range tc.expectedFiles {
				if manifestFile.Files[name].Size != size {
					t.Fatalf("expected filesystem manifest size %d for %s, got %+v", size, name, manifestFile.Files[name])
				}
				if cachedManifest.Files[name].Size != size {
					t.Fatalf("expected cached manifest size %d for %s, got %+v", size, name, cachedManifest.Files[name])
				}
			}
		})
	}
}
