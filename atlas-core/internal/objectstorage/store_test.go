package objectstorage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/anomalyco/atlas-core/internal/config"
	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/model"
)

func testLogger() *logging.Logger {
	cfg := &config.Config{LogLevel: "debug"}
	return logging.New(cfg, "test")
}

func initTestStore(t *testing.T) *Store {
	t.Helper()

	dir := t.TempDir()
	s := NewStore(dir, testLogger())
	if err := s.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
	return s
}

func initTestObjectFolder(t *testing.T) *Store {
	t.Helper()

	s := initTestStore(t)
	if err := s.CreateObjectFolder("obj_test"); err != nil {
		t.Fatalf("CreateObjectFolder failed: %v", err)
	}
	return s
}

func TestInitRoot_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "objects")

	s := NewStore(root, testLogger())
	if err := s.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}

	if !s.RootExists() {
		t.Fatal("root should exist after InitRoot")
	}
}

func TestCreateObjectFolder_CreatesFolderAndManifest(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, testLogger())

	if err := s.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}

	if err := s.CreateObjectFolder("obj_test"); err != nil {
		t.Fatalf("CreateObjectFolder failed: %v", err)
	}

	exists, err := s.ObjectFolderExists("obj_test")
	if err != nil {
		t.Fatalf("ObjectFolderExists failed: %v", err)
	}
	if !exists {
		t.Fatal("object folder should exist")
	}

	data, err := s.ReadManifestFile("obj_test")
	if err != nil {
		t.Fatalf("ReadManifestFile failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("manifest should not be empty")
	}
}

func TestWriteReadDeleteObjectFile(t *testing.T) {
	s := initTestObjectFolder(t)

	content := []byte("hello world")
	if err := s.WriteObjectFile("obj_test", "test.txt", content); err != nil {
		t.Fatalf("WriteObjectFile failed: %v", err)
	}

	data, err := s.ReadObjectFile("obj_test", "test.txt")
	if err != nil {
		t.Fatalf("ReadObjectFile failed: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", string(data))
	}

	if err := s.DeleteObjectFile("obj_test", "test.txt"); err != nil {
		t.Fatalf("DeleteObjectFile failed: %v", err)
	}

	_, err = s.ReadObjectFile("obj_test", "test.txt")
	if err == nil {
		t.Fatal("expected error reading deleted file")
	}
}

func TestAppendObjectFile(t *testing.T) {
	s := initTestObjectFolder(t)

	if err := s.WriteObjectFile("obj_test", "log.txt", []byte("line1\n")); err != nil {
		t.Fatalf("WriteObjectFile failed: %v", err)
	}
	if err := s.AppendObjectFile("obj_test", "log.txt", []byte("line2\n")); err != nil {
		t.Fatalf("AppendObjectFile failed: %v", err)
	}

	data, err := s.ReadObjectFile("obj_test", "log.txt")
	if err != nil {
		t.Fatalf("ReadObjectFile failed: %v", err)
	}
	if string(data) != "line1\nline2\n" {
		t.Fatalf("expected 'line1\\nline2\\n', got '%s'", string(data))
	}
}

func TestListObjectFolderFiles(t *testing.T) {
	s := initTestObjectFolder(t)

	if err := s.WriteObjectFile("obj_test", "a.txt", []byte("a")); err != nil {
		t.Fatalf("WriteObjectFile a.txt failed: %v", err)
	}
	if err := s.WriteObjectFile("obj_test", "b.txt", []byte("b")); err != nil {
		t.Fatalf("WriteObjectFile b.txt failed: %v", err)
	}

	files, err := s.ListObjectFolderFiles("obj_test")
	if err != nil {
		t.Fatalf("ListObjectFolderFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestDeleteObjectFolder(t *testing.T) {
	s := initTestObjectFolder(t)
	if err := s.WriteObjectFile("obj_test", "data.txt", []byte("data")); err != nil {
		t.Fatalf("WriteObjectFile failed: %v", err)
	}

	if err := s.DeleteObjectFolder("obj_test"); err != nil {
		t.Fatalf("DeleteObjectFolder failed: %v", err)
	}

	exists, err := s.ObjectFolderExists("obj_test")
	if err != nil {
		t.Fatalf("ObjectFolderExists failed: %v", err)
	}
	if exists {
		t.Fatal("object folder should not exist after delete")
	}
}

func TestDeleteObjectFolder_AllowsSafeNonObjectDirectoryNames(t *testing.T) {
	s := initTestStore(t)
	path := filepath.Join(s.root, "backup.2025-05-04")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir backup folder: %v", err)
	}

	if err := s.DeleteObjectFolder("backup.2025-05-04"); err != nil {
		t.Fatalf("DeleteObjectFolder failed: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected backup folder to be removed, got err=%v", err)
	}
}

func TestDeleteObjectFolder_StaysRootedWhenRootPathIsReplaced(t *testing.T) {
	parent := t.TempDir()
	rootPath := filepath.Join(parent, "objects")
	s := NewStore(rootPath, testLogger())
	if err := s.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}
	defer s.Close()
	if err := s.CreateObjectFolder("obj_test"); err != nil {
		t.Fatalf("CreateObjectFolder failed: %v", err)
	}
	if err := s.WriteObjectFile("obj_test", "data.txt", []byte("inside")); err != nil {
		t.Fatalf("WriteObjectFile failed: %v", err)
	}

	realRoot := filepath.Join(parent, "objects-real")
	if err := os.Rename(rootPath, realRoot); err != nil {
		t.Fatalf("rename root failed: %v", err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(filepath.Join(outside, "obj_test"), 0o700); err != nil {
		t.Fatalf("mkdir outside object failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "obj_test", "data.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside file failed: %v", err)
	}
	if err := os.Symlink(outside, rootPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if err := s.DeleteObjectFolder("obj_test"); err != nil {
		t.Fatalf("DeleteObjectFolder failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(realRoot, "obj_test")); !os.IsNotExist(err) {
		t.Fatalf("expected original rooted object folder to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "obj_test", "data.txt")); err != nil {
		t.Fatalf("expected outside path to remain untouched, got err=%v", err)
	}
}

func TestValidateSafeObjectPath(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, testLogger())

	tests := []struct {
		objectID string
		filename string
		valid    bool
	}{
		{"obj", "file.txt", true},
		{"", "file.txt", false},
		{".", "file.txt", false},
		{"manifest.json", "file.txt", false},
		{"obj", "", false},
		{"obj..", "file.txt", false},
		{"obj", ".", false},
		{"obj", "..file", false},
		{"obj/sub", "file.txt", false},
		{"obj\\sub", "file.txt", false},
		{"obj", "sub/file.txt", false},
		{"obj", "sub\\file.txt", false},
	}

	for _, tt := range tests {
		err := s.ValidateSafeObjectPath(tt.objectID, tt.filename)
		if tt.valid && err != nil {
			t.Errorf("expected valid path %q/%q, got error: %v", tt.objectID, tt.filename, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("expected invalid path %q/%q, got no error", tt.objectID, tt.filename)
		}
	}
}

func TestValidateObjectID(t *testing.T) {
	tests := []struct {
		objectID string
		valid    bool
	}{
		{"obj_test", true},
		{"obj-test", true},
		{"", false},
		{".", false},
		{"..", false},
		{"manifest.json", false},
		{"obj.with.dot", false},
		{"obj/test", false},
	}

	for _, tt := range tests {
		err := ValidateObjectID(tt.objectID)
		if tt.valid && err != nil {
			t.Errorf("expected valid object_id %q, got error: %v", tt.objectID, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("expected invalid object_id %q, got no error", tt.objectID)
		}
	}
}

func TestReadManifestWriteManifest(t *testing.T) {
	s := initTestObjectFolder(t)

	manifest := []byte(`{"files":{"data.txt":{"size":5,"updated_at":"2025-01-01T00:00:00Z"}}}`)
	if err := s.WriteManifestFile("obj_test", manifest); err != nil {
		t.Fatalf("WriteManifestFile failed: %v", err)
	}

	data, err := s.ReadManifestFile("obj_test")
	if err != nil {
		t.Fatalf("ReadManifestFile failed: %v", err)
	}
	if string(data) != string(manifest) {
		t.Fatalf("manifest mismatch: got %s, expected %s", string(data), string(manifest))
	}
}

func TestReaderForObjectFile(t *testing.T) {
	s := initTestObjectFolder(t)
	if err := s.WriteObjectFile("obj_test", "data.txt", []byte("streaming data")); err != nil {
		t.Fatalf("WriteObjectFile failed: %v", err)
	}

	reader, err := s.ReaderForObjectFile("obj_test", "data.txt")
	if err != nil {
		t.Fatalf("ReaderForObjectFile failed: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "streaming data" {
		t.Fatalf("expected 'streaming data', got '%s'", string(data))
	}
}

func TestInitRoot_SecondCallSucceeds(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "objects")

	s := NewStore(root, testLogger())
	if err := s.InitRoot(); err != nil {
		t.Fatalf("first InitRoot failed: %v", err)
	}
	if err := s.InitRoot(); err != nil {
		t.Fatalf("second InitRoot failed: %v", err)
	}
}

func TestObjectFolderExists_NonExistent(t *testing.T) {
	s := initTestStore(t)

	exists, err := s.ObjectFolderExists("nonexistent")
	if err != nil {
		t.Fatalf("ObjectFolderExists failed: %v", err)
	}
	if exists {
		t.Fatal("nonexistent folder should not exist")
	}
}

func TestReadManifestFile_NonExistent(t *testing.T) {
	s := initTestObjectFolder(t)

	// Delete the manifest to trigger not found
	if err := os.Remove(filepath.Join(s.root, "obj_test", manifestFilename)); err != nil {
		t.Fatalf("Remove manifest failed: %v", err)
	}

	_, err := s.ReadManifestFile("obj_test")
	if err == nil {
		t.Fatal("expected error reading nonexistent manifest")
	}
}

func TestWriteManifestFile_RejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	s := NewStore(dir, testLogger())
	if err := s.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}

	linkPath := filepath.Join(dir, "obj_link")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if err := s.WriteManifestFile("obj_link", []byte(`{"files":{}}`)); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestWriteObjectFile_RejectsSymlinkFile(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	s := NewStore(dir, testLogger())
	if err := s.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}
	if err := s.CreateObjectFolder("obj_test"); err != nil {
		t.Fatalf("CreateObjectFolder failed: %v", err)
	}

	linkPath := filepath.Join(dir, "obj_test", "data.txt")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if err := s.WriteObjectFile("obj_test", "data.txt", []byte("test")); err == nil {
		t.Fatal("expected symlink file write to be rejected")
	}
}

func TestSafeOpenAt_RejectsIntermediateSymlink(t *testing.T) {
	// The classic gap: an attacker plants a symlink at a non-leaf path
	// component. A naive O_NOFOLLOW-on-leaf design would still follow the
	// intermediate symlink because the kernel resolves the path components in
	// order. The per-component walk in safeOpenAt opens each level with
	// O_NOFOLLOW, so the symlink is detected at the level it appears.
	dir := t.TempDir()
	s := NewStore(dir, testLogger())
	if err := s.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}
	defer s.Close()

	// Plant a real directory outside the storage root with a file in it.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "data.txt"), []byte("escape"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	// Symlink an "object folder" inside the storage root to the outside dir.
	if err := os.Symlink(outside, filepath.Join(dir, "obj_link")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	// Reads, writes, and listings that name the symlinked folder must all be
	// refused — the intermediate symlink is detected during the walk.
	if _, err := s.ReadObjectFile("obj_link", "data.txt"); err == nil {
		t.Fatal("expected ReadObjectFile through symlinked intermediate to fail")
	}
	if err := s.WriteObjectFile("obj_link", "data.txt", []byte("x")); err == nil {
		t.Fatal("expected WriteObjectFile through symlinked intermediate to fail")
	}
	if err := s.AppendObjectFile("obj_link", "data.txt", []byte("x")); err == nil {
		t.Fatal("expected AppendObjectFile through symlinked intermediate to fail")
	}
	if _, err := s.ListObjectFolderFiles("obj_link"); err == nil {
		t.Fatal("expected ListObjectFolderFiles through symlinked intermediate to fail")
	}
	if _, err := s.ReadManifestFile("obj_link"); err == nil {
		t.Fatal("expected ReadManifestFile through symlinked intermediate to fail")
	}
}

func TestWriteObjectFile_RejectsSymlinkObjectFolder(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	s := NewStore(dir, testLogger())
	if err := s.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}
	defer s.Close()

	linkPath := filepath.Join(dir, "obj_link")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	// Object folder is itself a symlink to outside the storage root. Writes
	// targeted at "obj_link/data.txt" must not follow the link.
	if err := s.WriteObjectFile("obj_link", "data.txt", []byte("test")); err == nil {
		t.Fatal("expected write through symlinked object folder to be rejected")
	}
	if _, err := s.ReadObjectFile("obj_link", "data.txt"); err == nil {
		t.Fatal("expected read through symlinked object folder to be rejected")
	}
	if err := s.AppendObjectFile("obj_link", "data.txt", []byte("more")); err == nil {
		t.Fatal("expected append through symlinked object folder to be rejected")
	}
}

func TestGenericObjectFileAPIs_ReserveManifestFile(t *testing.T) {
	s := initTestObjectFolder(t)

	if err := s.WriteObjectFile("obj_test", manifestFilename, []byte(`{}`)); err == nil {
		t.Fatal("expected manifest filename write to be rejected")
	}
	if _, err := s.ReadObjectFile("obj_test", manifestFilename); err == nil {
		t.Fatal("expected manifest filename read to be rejected")
	}
	if err := s.DeleteObjectFile("obj_test", manifestFilename); err == nil {
		t.Fatal("expected manifest filename delete to be rejected")
	}
}

func TestCreateObjectFolder_AlreadyExists(t *testing.T) {
	s := initTestStore(t)

	if err := s.CreateObjectFolder("obj_test"); err != nil {
		t.Fatalf("first CreateObjectFolder failed: %v", err)
	}
	// Second call should succeed (just write manifest again)
	if err := s.CreateObjectFolder("obj_test"); err != nil {
		t.Fatalf("second CreateObjectFolder failed: %v", err)
	}
}

func TestMissingObjectFolderReturnsNotFound(t *testing.T) {
	s := initTestStore(t)

	if _, err := s.ReadManifestFile("missing_obj"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected manifest read to return model.ErrNotFound, got %v", err)
	}
	if _, err := s.ReadObjectFile("missing_obj", "data.txt"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected object file read to return model.ErrNotFound, got %v", err)
	}
	if _, err := s.ListObjectFolderFiles("missing_obj"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected object file listing to return model.ErrNotFound, got %v", err)
	}
}

func TestWithObjectLock_ReleasesUnusedLocks(t *testing.T) {
	s := initTestStore(t)

	if err := s.withObjectLock("obj_test", func() error { return nil }); err != nil {
		t.Fatalf("withObjectLock failed: %v", err)
	}
	if got := len(s.locks); got != 0 {
		t.Fatalf("expected lock map to be empty after use, got %d entries", got)
	}
}

func TestReadManifestFile_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, testLogger())
	if err := s.InitRoot(); err != nil {
		t.Fatalf("InitRoot failed: %v", err)
	}
	defer s.Close()

	if err := s.CreateObjectFolder("obj_link"); err != nil {
		t.Fatalf("CreateObjectFolder failed: %v", err)
	}

	// Replace the manifest with a symlink pointing at a real file outside the
	// object dir. The safe-walk Read path must refuse to follow it.
	manifestPath := filepath.Join(dir, "obj_link", manifestFilename)
	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("remove manifest: %v", err)
	}
	target := filepath.Join(t.TempDir(), "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, manifestPath); err != nil {
		t.Fatalf("symlink manifest: %v", err)
	}

	if _, err := s.ReadManifestFile("obj_link"); err == nil {
		t.Fatal("expected symlink manifest read to fail")
	}
}
