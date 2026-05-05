package app

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/config"
	"github.com/anomalyco/atlas-core/internal/function"
	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/objectstorage"
	"github.com/anomalyco/atlas-core/internal/store"
)

func TestRemoveReadyFile_IgnoresMissingFile(t *testing.T) {
	if err := removeReadyFile(filepath.Join(t.TempDir(), ".ready")); err != nil {
		t.Fatalf("removeReadyFile returned error for missing file: %v", err)
	}
}

func TestRemoveReadyFile_RemovesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ready")
	if err := os.WriteFile(path, []byte("ready\n"), 0o644); err != nil {
		t.Fatalf("write ready file: %v", err)
	}

	if err := removeReadyFile(path); err != nil {
		t.Fatalf("removeReadyFile returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected ready file to be removed, got err=%v", err)
	}
}

type blockingObjectStore struct {
	started chan struct{}
	release chan struct{}
}

func (s blockingObjectStore) CreateObject(context.Context, *model.Object) error { return nil }
func (s blockingObjectStore) GetObject(context.Context, string) (*model.Object, error) {
	return nil, model.ErrNotFound
}
func (s blockingObjectStore) ListObjects(context.Context, ...store.ObjectFilter) ([]model.Object, error) {
	select {
	case <-s.started:
	default:
		close(s.started)
	}
	<-s.release
	return nil, nil
}
func (s blockingObjectStore) UpdateObject(context.Context, *model.Object) error { return nil }
func (s blockingObjectStore) DeleteObject(context.Context, string) error        { return nil }
func (s blockingObjectStore) UpsertObject(context.Context, *model.Object) error { return nil }
func (s blockingObjectStore) UpdateObjectManifest(context.Context, string, *model.ObjectManifest, ...time.Time) error {
	return nil
}
func (s blockingObjectStore) GetObjectManifest(context.Context, string) (*model.ObjectManifest, error) {
	return model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}), nil
}

type noopObjectStorage struct{}

func (noopObjectStorage) CreateObjectFolder(string) error                { return nil }
func (noopObjectStorage) ObjectFolderExists(string) (bool, error)        { return false, nil }
func (noopObjectStorage) ListObjectFolders() ([]string, error)           { return nil, nil }
func (noopObjectStorage) DeleteObjectFolder(string) error                { return nil }
func (noopObjectStorage) WriteObjectFile(string, string, []byte) error   { return nil }
func (noopObjectStorage) AppendObjectFile(string, string, []byte) error  { return nil }
func (noopObjectStorage) ReadObjectFile(string, string) ([]byte, error)  { return nil, nil }
func (noopObjectStorage) DeleteObjectFile(string, string) error          { return nil }
func (noopObjectStorage) ListObjectFolderFiles(string) ([]string, error) { return nil, nil }
func (noopObjectStorage) GetObjectFileInfo(string, string) (model.ObjectFileInfo, error) {
	return model.ObjectFileInfo{}, nil
}
func (noopObjectStorage) ReadManifestFile(string) ([]byte, error) {
	return json.Marshal(model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}))
}
func (noopObjectStorage) WriteManifestFile(string, []byte) error                    { return nil }
func (noopObjectStorage) ValidateSafeObjectPath(string, string) error               { return nil }
func (noopObjectStorage) ReaderForObjectFile(string, string) (io.ReadCloser, error) { return nil, nil }

type noopIdempotencyStore struct{}

func (noopIdempotencyStore) TryBegin(context.Context, string, string, string) (store.IdempotencyRecord, bool, error) {
	return store.IdempotencyRecord{}, true, nil
}
func (noopIdempotencyStore) MarkCompleted(context.Context, string, string) error { return nil }
func (noopIdempotencyStore) MarkFailed(context.Context, string, string) error    { return nil }

func TestShutdown_WaitsForReconciler(t *testing.T) {
	logger := logging.New(&config.Config{LogLevel: "debug"}, "test")
	started := make(chan struct{})
	release := make(chan struct{})
	app := &App{
		Config: &config.Config{
			ReconcileInterval: 10 * time.Millisecond,
			ReconcileTimeout:  time.Second,
			ReadyFile:         filepath.Join(t.TempDir(), ".ready"),
		},
		Logger: logger,
		Funcs: function.Functions{
			Object: function.NewObjectFunctions(blockingObjectStore{started: started, release: release}, noopObjectStorage{}, noopIdempotencyStore{}, logger),
		},
	}

	app.startReconciler()
	<-started

	done := make(chan struct{})
	go func() {
		app.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Shutdown returned before in-flight reconcile finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not complete after reconcile finished")
	}
}

func TestCloseStartupResources_ClosesObjectStore(t *testing.T) {
	logger := logging.New(&config.Config{LogLevel: "debug"}, "test")
	objStore := objectstorage.NewStore(t.TempDir(), logger)
	if err := objStore.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}

	closeStartupResources(nil, objStore, logger)

	if _, err := objStore.ObjectFolderExists("obj_test"); err == nil {
		t.Fatal("expected closed object store to reject new operations")
	}
}
