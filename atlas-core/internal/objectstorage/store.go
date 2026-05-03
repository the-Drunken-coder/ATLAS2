package objectstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/model"
)

type Store struct {
	root string
	log  *logging.Logger
}

func NewStore(root string, log *logging.Logger) *Store {
	return &Store{root: root, log: log}
}

func (s *Store) InitRoot() error {
	s.log.Info("object_storage", fmt.Sprintf("initializing object storage at %s", s.root))
	if err := os.MkdirAll(s.root, 0755); err != nil {
		return fmt.Errorf("create object storage root: %w", err)
	}
	s.log.Info("object_storage", "object storage root initialized")
	return nil
}

func (s *Store) RootExists() bool {
	info, err := os.Stat(s.root)
	return err == nil && info.IsDir()
}

func (s *Store) CreateObjectFolder(objectID string) error {
	path, err := s.objectPath(objectID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("create object folder %s: %w", objectID, err)
	}
	manifestPath := filepath.Join(path, "manifest.json")
	if _, err := os.Stat(manifestPath); errors.Is(err, os.ErrNotExist) {
		emptyManifest := model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}}
		return s.WriteManifestFile(objectID, mustMarshal(emptyManifest))
	}
	return nil
}

func (s *Store) ObjectFolderExists(objectID string) (bool, error) {
	path, err := s.objectPath(objectID)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("check object folder %s: %w", objectID, err)
	}
	return info.IsDir(), nil
}

func (s *Store) DeleteObjectFolder(objectID string) error {
	path, err := s.objectPath(objectID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("delete object folder %s: %w", objectID, err)
	}
	return nil
}

func (s *Store) WriteObjectFile(objectID, filename string, data []byte) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	path, err := s.filePath(objectID, filename)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write object file %s/%s: %w", objectID, filename, err)
	}
	return nil
}

func (s *Store) AppendObjectFile(objectID, filename string, data []byte) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	path, err := s.filePath(objectID, filename)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("append object file %s/%s: %w", objectID, filename, err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write append data %s/%s: %w", objectID, filename, err)
	}
	return nil
}

func (s *Store) ReadObjectFile(objectID, filename string) ([]byte, error) {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, err
	}
	path, err := s.filePath(objectID, filename)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("read object file %s/%s: %w", objectID, filename, err)
	}
	return data, nil
}

func (s *Store) DeleteObjectFile(objectID, filename string) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	path, err := s.filePath(objectID, filename)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ErrNotFound
		}
		return fmt.Errorf("delete object file %s/%s: %w", objectID, filename, err)
	}
	return nil
}

func (s *Store) ListObjectFolderFiles(objectID string) ([]string, error) {
	path, err := s.objectPath(objectID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("list object folder %s: %w", objectID, err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func (s *Store) ReadManifestFile(objectID string) ([]byte, error) {
	path, err := s.manifestPath(objectID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("read manifest for %s: %w", objectID, err)
	}
	return data, nil
}

func (s *Store) WriteManifestFile(objectID string, data []byte) error {
	path, err := s.manifestPath(objectID)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write manifest for %s: %w", objectID, err)
	}
	return nil
}

func (s *Store) ValidateSafeObjectPath(objectID, filename string) error {
	if err := ValidateObjectID(objectID); err != nil {
		return err
	}
	if filename == "" {
		return fmt.Errorf("filename is required")
	}
	if strings.Contains(filename, "..") {
		return fmt.Errorf("invalid path: object_id or filename contains '..'")
	}
	if strings.ContainsAny(filename, `/\`) {
		return fmt.Errorf("invalid path: object_id or filename contains path separators")
	}
	if filepath.IsAbs(filename) {
		return fmt.Errorf("invalid path: object_id or filename must be relative")
	}
	return nil
}

func (s *Store) ReaderForObjectFile(objectID, filename string) (io.ReadCloser, error) {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, err
	}
	path, err := s.filePath(objectID, filename)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("open object file %s/%s: %w", objectID, filename, err)
	}
	return f, nil
}

func ValidateObjectID(objectID string) error {
	if objectID == "" {
		return fmt.Errorf("object_id is required")
	}
	if strings.Contains(objectID, "..") {
		return fmt.Errorf("invalid path: object_id or filename contains '..'")
	}
	if strings.ContainsAny(objectID, `/\`) {
		return fmt.Errorf("invalid path: object_id or filename contains path separators")
	}
	if filepath.IsAbs(objectID) {
		return fmt.Errorf("invalid path: object_id or filename must be relative")
	}
	return nil
}

func (s *Store) objectPath(objectID string) (string, error) {
	if err := ValidateObjectID(objectID); err != nil {
		return "", err
	}
	return safeJoinUnderRoot(s.root, objectID)
}

func (s *Store) filePath(objectID, filename string) (string, error) {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return "", err
	}
	return safeJoinUnderRoot(s.root, objectID, filename)
}

func (s *Store) manifestPath(objectID string) (string, error) {
	if err := ValidateObjectID(objectID); err != nil {
		return "", err
	}
	return safeJoinUnderRoot(s.root, objectID, "manifest.json")
}

func safeJoinUnderRoot(root string, parts ...string) (string, error) {
	base := filepath.Clean(root)
	candidate := filepath.Join(append([]string{base}, parts...)...)
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path under root: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path: resolved path escapes storage root")
	}
	return candidate, nil
}

func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal manifest: %v", err))
	}
	return data
}
