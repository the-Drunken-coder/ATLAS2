package app

import (
	"os"
	"path/filepath"
	"testing"
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
