package objectstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/model"
)

type Store struct {
	root  string
	log   *logging.Logger
	locks sync.Map
}

func NewStore(root string, log *logging.Logger) *Store {
	return &Store{root: filepath.Clean(root), log: log}
}

func (s *Store) InitRoot() error {
	s.log.Info("object_storage", "initializing object storage", logging.String("root", s.root))
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("create object storage root: %w", err)
	}
	if err := ensureDirectoryPath(s.root); err != nil {
		return fmt.Errorf("validate object storage root: %w", err)
	}
	s.log.Info("object_storage", "object storage root initialized", logging.String("root", s.root))
	return nil
}

func (s *Store) RootExists() bool {
	info, err := os.Stat(s.root)
	return err == nil && info.IsDir()
}

func (s *Store) CreateObjectFolder(objectID string) error {
	if err := ValidateObjectID(objectID); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		path := filepath.Join(s.root, objectID)
		if err := os.Mkdir(path, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create object folder %s: %w", objectID, err)
		}
		if err := ensureDirectoryPath(path); err != nil {
			return fmt.Errorf("validate object folder %s: %w", objectID, err)
		}
		manifestPath := filepath.Join(path, manifestFilename)
		data, err := readExistingManifest(manifestPath)
		if err == nil {
			var manifest model.ObjectManifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return fmt.Errorf("validate existing manifest for %s: %w", objectID, err)
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat manifest for %s: %w", objectID, err)
		}
		emptyManifest := model.NormalizeManifest(&model.ObjectManifest{Files: map[string]model.ObjectFileInfo{}})
		manifestData, err := json.Marshal(emptyManifest)
		if err != nil {
			return fmt.Errorf("marshal manifest for %s: %w", objectID, err)
		}
		return s.writeManifestFileUnlocked(objectID, manifestData)
	})
}

func (s *Store) ObjectFolderExists(objectID string) (bool, error) {
	if err := ValidateObjectID(objectID); err != nil {
		return false, err
	}
	path := filepath.Join(s.root, objectID)
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("check object folder %s: %w", objectID, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("invalid path: object folder resolves through a symlink")
	}
	return info.IsDir(), nil
}

func (s *Store) ListObjectFolders() ([]string, error) {
	if err := ensureDirectoryPath(s.root); err != nil {
		return nil, fmt.Errorf("validate object storage root: %w", err)
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("list object folders: %w", err)
	}
	folders := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("invalid path: object folder %s resolves through a symlink", entry.Name())
		}
		if entry.IsDir() {
			folders = append(folders, entry.Name())
		}
	}
	return folders, nil
}

func (s *Store) DeleteObjectFolder(objectID string) error {
	if err := ValidateObjectID(objectID); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		path := filepath.Join(s.root, objectID)
		if err := ensureDirectoryPath(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("validate object folder %s: %w", objectID, err)
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("delete object folder %s: %w", objectID, err)
		}
		return syncDir(s.root)
	})
}

func (s *Store) WriteObjectFile(objectID, filename string, data []byte) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		path, err := s.filePath(objectID, filename)
		if err != nil {
			return err
		}
		if err := writeFileNoFollow(path, data, 0o644); err != nil {
			return fmt.Errorf("write object file %s/%s: %w", objectID, filename, err)
		}
		return nil
	})
}

func (s *Store) AppendObjectFile(objectID, filename string, data []byte) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		path, err := s.filePath(objectID, filename)
		if err != nil {
			return err
		}
		f, err := openFileNoFollow(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("append object file %s/%s: %w", objectID, filename, err)
		}
		defer f.Close()
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("write append data %s/%s: %w", objectID, filename, err)
		}
		return f.Sync()
	})
}

func (s *Store) ReadObjectFile(objectID, filename string) ([]byte, error) {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, err
	}
	path, err := s.filePath(objectID, filename)
	if err != nil {
		return nil, err
	}
	data, err := readFileNoFollow(path)
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
	return s.withObjectLock(objectID, func() error {
		path, err := s.filePath(objectID, filename)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return model.ErrNotFound
			}
			return fmt.Errorf("stat object file %s/%s: %w", objectID, filename, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("invalid path: object file resolves through a symlink")
		}
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return model.ErrNotFound
			}
			return fmt.Errorf("delete object file %s/%s: %w", objectID, filename, err)
		}
		return syncDir(filepath.Dir(path))
	})
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
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("invalid path: object file %s resolves through a symlink", entry.Name())
		}
		if !entry.IsDir() && entry.Name() != manifestFilename {
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
	data, err := readFileNoFollow(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("read manifest for %s: %w", objectID, err)
	}
	return data, nil
}

func (s *Store) WriteManifestFile(objectID string, data []byte) error {
	if err := ValidateObjectID(objectID); err != nil {
		return err
	}
	if !json.Valid(data) {
		return fmt.Errorf("manifest must be valid JSON")
	}
	return s.withObjectLock(objectID, func() error {
		return s.writeManifestFileUnlocked(objectID, data)
	})
}

func (s *Store) ValidateSafeObjectPath(objectID, filename string) error {
	if err := ValidateObjectID(objectID); err != nil {
		return err
	}
	if filename == "" {
		return fmt.Errorf("filename is required")
	}
	if filename == "." || filename == ".." {
		return fmt.Errorf("invalid path: filename must not be '.' or '..'")
	}
	if strings.Contains(filename, "..") {
		return fmt.Errorf("invalid path: filename contains '..'")
	}
	if filepath.IsAbs(filename) {
		return fmt.Errorf("invalid path: filename must be relative")
	}
	if strings.ContainsAny(filename, `/\\`) {
		return fmt.Errorf("invalid path: filename contains path separators")
	}
	if filename == manifestFilename {
		return fmt.Errorf("invalid path: filename is reserved")
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
	f, err := openFileNoFollow(path, os.O_RDONLY, 0)
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
	if objectID == "." || objectID == ".." {
		return fmt.Errorf("invalid path: object_id must not be '.' or '..'")
	}
	if objectID == manifestFilename {
		return fmt.Errorf("invalid path: object_id is reserved")
	}
	if filepath.IsAbs(objectID) || strings.ContainsAny(objectID, `/\\`) {
		return fmt.Errorf("invalid path: object_id contains path separators")
	}
	if !objectIDPattern.MatchString(objectID) {
		return fmt.Errorf("invalid path: object_id must use only letters, numbers, '_' or '-'")
	}
	return nil
}

func (s *Store) objectPath(objectID string) (string, error) {
	if err := ValidateObjectID(objectID); err != nil {
		return "", err
	}
	path := filepath.Join(s.root, objectID)
	if err := ensureDirectoryPath(path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) filePath(objectID, filename string) (string, error) {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return "", err
	}
	objectPath, err := s.objectPath(objectID)
	if err != nil {
		return "", err
	}
	return filepath.Join(objectPath, filename), nil
}

func (s *Store) manifestPath(objectID string) (string, error) {
	objectPath, err := s.objectPath(objectID)
	if err != nil {
		return "", err
	}
	return filepath.Join(objectPath, manifestFilename), nil
}

func (s *Store) writeManifestFileUnlocked(objectID string, data []byte) error {
	objectPath, err := s.objectPath(objectID)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(objectPath, manifestFilename)
	tmpPath := filepath.Join(objectPath, fmt.Sprintf(".%s.tmp-%d", manifestFilename, time.Now().UTC().UnixNano()))
	f, err := openFileNoFollow(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create manifest temp file for %s: %w", objectID, err)
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write manifest temp file for %s: %w", objectID, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync manifest temp file for %s: %w", objectID, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close manifest temp file for %s: %w", objectID, err)
	}
	if err := os.Rename(tmpPath, manifestPath); err != nil {
		return fmt.Errorf("rename manifest for %s: %w", objectID, err)
	}
	cleanupTemp = false
	if err := syncDir(objectPath); err != nil {
		return fmt.Errorf("sync object folder for %s: %w", objectID, err)
	}
	return nil
}

func (s *Store) withObjectLock(objectID string, fn func() error) error {
	muIface, _ := s.locks.LoadOrStore(objectID, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

func ensureDirectoryPath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("invalid path: %s resolves through a symlink", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("invalid path: %s is not a directory", path)
	}
	return nil
}

func readExistingManifest(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("invalid path: manifest resolves through a symlink")
	}
	return os.ReadFile(path)
}

func readFileNoFollow(path string) ([]byte, error) {
	f, err := openFileNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func writeFileNoFollow(path string, data []byte, perm os.FileMode) error {
	f, err := openFileNoFollow(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return nil
}

const manifestFilename = "manifest.json"

var objectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
