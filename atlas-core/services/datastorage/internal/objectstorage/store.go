package objectstorage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/objectpath"
)

type AppendSizePreconditionError struct {
	ActualSize   int64
	ExpectedSize int64
}

func (e *AppendSizePreconditionError) Error() string {
	return fmt.Sprintf("append precondition failed: actual size %d does not match expected size %d", e.ActualSize, e.ExpectedSize)
}

type Store struct {
	root    string
	log     *logging.Logger
	rootFD  *os.File
	locksMu sync.Mutex
	locks   map[string]*objectLock
}

func NewStore(root string, log *logging.Logger) *Store {
	return &Store{
		root:  filepath.Clean(root),
		log:   log,
		locks: make(map[string]*objectLock),
	}
}

func (s *Store) InitRoot() error {
	s.log.Info("object_storage", "initializing object storage", logging.String("root", s.root))
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create object storage root: %w", err)
	}
	if err := ensureDirectoryPath(s.root); err != nil {
		return fmt.Errorf("validate object storage root: %w", err)
	}
	// Open the root once; subsequent file operations are rooted at this FD so
	// later symlink replacements at the path-name level cannot redirect them.
	rootFD, err := openRootPathNoFollow(s.root)
	if err != nil {
		return fmt.Errorf("open object storage root: %w", err)
	}
	defer func() {
		if rootFD != nil {
			if closeErr := rootFD.Close(); closeErr != nil {
				s.log.Warn("object_storage", "failed to close unassigned object storage root fd", logging.ErrorField(closeErr))
			}
		}
	}()
	if s.rootFD != nil {
		if err := s.rootFD.Close(); err != nil {
			return fmt.Errorf("close previous object storage root: %w", err)
		}
	}
	s.rootFD = rootFD
	rootFD = nil
	s.log.Info("object_storage", "object storage root initialized", logging.String("root", s.root))
	return nil
}

// Close releases the storage root FD held since InitRoot. Safe to call on a
// store that was never initialized.
func (s *Store) Close() error {
	if s.rootFD == nil {
		return nil
	}
	err := s.rootFD.Close()
	s.rootFD = nil
	return err
}

func (s *Store) RootExists() bool {
	info, err := os.Stat(s.root)
	return err == nil && info.IsDir()
}

func (s *Store) CreateObjectFolder(objectID string) error {
	if err := objectpath.ValidateObjectID(objectID); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		if err := s.requireRoot(); err != nil {
			return err
		}
		if err := safeMkdirAt(s.rootFD, []string{objectID}, 0o700); err != nil {
			if !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("create object folder %s: %w", objectID, err)
			}
		} else {
			if err := s.fsyncRoot(); err != nil {
				return fmt.Errorf("sync object storage root after creating %s: %w", objectID, err)
			}
		}
		// Validate the leaf isn't a symlink even on the "already exists" branch.
		dir, err := safeOpenAt(s.rootFD, []string{objectID}, os.O_RDONLY|openDirectoryFlag, 0)
		if err != nil {
			return fmt.Errorf("validate object folder %s: %w", objectID, err)
		}
		dir.Close()

		data, err := s.readManifestUnlocked(objectID)
		if err == nil {
			var manifest model.ObjectManifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return fmt.Errorf("validate existing manifest for %s: %w", objectID, err)
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, model.ErrNotFound) {
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
	if err := objectpath.ValidateObjectID(objectID); err != nil {
		return false, err
	}
	if err := s.requireRoot(); err != nil {
		return false, err
	}
	dir, err := safeOpenAt(s.rootFD, []string{objectID}, os.O_RDONLY|openDirectoryFlag, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("check object folder %s: %w", objectID, err)
	}
	dir.Close()
	return true, nil
}

func (s *Store) ListObjectFolders() ([]string, error) {
	if err := s.requireRoot(); err != nil {
		return nil, err
	}
	// Stat the root through the FD so we don't walk the path string again.
	if _, err := s.rootFD.Stat(); err != nil {
		return nil, fmt.Errorf("validate object storage root: %w", err)
	}
	// Reading the directory through the FD avoids re-resolving the path.
	if _, err := s.rootFD.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind storage root: %w", err)
	}
	entries, err := s.rootFD.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("list object folders: %w", err)
	}
	folders := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("invalid path: object folder %s resolves through a symlink", entry.Name())
		}
		if entry.IsDir() {
			folders = append(folders, entry.Name())
		}
	}
	return folders, nil
}

func (s *Store) DeleteObjectFolder(objectID string) error {
	if err := objectpath.ValidateDeletableFolderName(objectID); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		if err := s.requireRoot(); err != nil {
			return err
		}
		// Validate the target is a real directory, not a symlink, before
		// recursively deleting through it.
		dir, err := safeOpenAt(s.rootFD, []string{objectID}, os.O_RDONLY|openDirectoryFlag, 0)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("validate object folder %s: %w", objectID, err)
		}
		dir.Close()
		if err := safeRemoveAllAt(s.rootFD, []string{objectID}); err != nil {
			return fmt.Errorf("delete object folder %s: %w", objectID, err)
		}
		return s.fsyncRoot()
	})
}

func (s *Store) RenameObjectFolder(objectID, newName string) error {
	if err := objectpath.ValidateObjectID(objectID); err != nil {
		return err
	}
	if err := objectpath.ValidateObjectID(newName); err != nil {
		return err
	}
	if objectID == newName {
		return nil
	}

	firstName, secondName := objectID, newName
	if secondName < firstName {
		firstName, secondName = secondName, firstName
	}
	firstLock := s.acquireObjectLock(firstName)
	firstLock.mu.Lock()
	defer func() {
		firstLock.mu.Unlock()
		s.releaseObjectLock(firstName, firstLock)
	}()

	// old->new renames only need one lock when both names are equal; otherwise
	// we lock in sorted order so concurrent renames cannot deadlock.
	secondLock := firstLock
	if secondName != firstName {
		secondLock = s.acquireObjectLock(secondName)
		secondLock.mu.Lock()
		defer func() {
			secondLock.mu.Unlock()
			s.releaseObjectLock(secondName, secondLock)
		}()
	}

	if err := s.requireRoot(); err != nil {
		return err
	}
	dir, err := safeOpenAt(s.rootFD, []string{objectID}, os.O_RDONLY|openDirectoryFlag, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ErrNotFound
		}
		return fmt.Errorf("validate object folder %s: %w", objectID, err)
	}
	dir.Close()
	if err := safeRenameAt(s.rootFD, []string{objectID}, []string{newName}); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ErrNotFound
		}
		return fmt.Errorf("rename object folder %s to %s: %w", objectID, newName, err)
	}
	return s.fsyncRoot()
}

func (s *Store) WriteObjectFile(objectID, filename string, data []byte) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		if err := s.requireRoot(); err != nil {
			return err
		}
		f, err := safeOpenAt(s.rootFD, []string{objectID, filename}, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return fmt.Errorf("write object file %s/%s: %w", objectID, filename, err)
		}
		defer f.Close()
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("write object file %s/%s: %w", objectID, filename, err)
		}
		if err := f.Sync(); err != nil {
			return fmt.Errorf("sync object file %s/%s: %w", objectID, filename, err)
		}
		if err := s.fsyncDirAt([]string{objectID}); err != nil {
			return fmt.Errorf("sync object folder %s after writing %s: %w", objectID, filename, err)
		}
		return nil
	})
}

func (s *Store) StreamWriteObjectFile(objectID, filename string, write func(io.Writer) error) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		return s.writeObjectFileStreamLocked(objectID, filename, write)
	})
}

func (s *Store) AppendObjectFile(objectID, filename string, data []byte) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		if err := s.requireRoot(); err != nil {
			return err
		}
		f, err := safeOpenAt(s.rootFD, []string{objectID, filename}, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("append object file %s/%s: %w", objectID, filename, err)
		}
		defer f.Close()
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("append object file %s/%s: %w", objectID, filename, err)
		}
		if err := f.Sync(); err != nil {
			return fmt.Errorf("sync object file %s/%s: %w", objectID, filename, err)
		}
		if err := s.fsyncDirAt([]string{objectID}); err != nil {
			return fmt.Errorf("sync object folder %s after appending %s: %w", objectID, filename, err)
		}
		return nil
	})
}

func (s *Store) StreamAppendObjectFile(objectID, filename string, currentExpectedSize int64, write func(io.Writer, int64) error) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		if err := s.requireRoot(); err != nil {
			return err
		}
		actualSize, err := s.currentObjectFileSizeLocked(objectID, filename)
		if err != nil {
			return err
		}
		if actualSize != currentExpectedSize {
			return &AppendSizePreconditionError{ActualSize: actualSize, ExpectedSize: currentExpectedSize}
		}
		return s.writeAppendedObjectFileLocked(objectID, filename, func(w io.Writer) error {
			return write(w, actualSize)
		})
	})
}

func (s *Store) ReadObjectFile(objectID, filename string) ([]byte, error) {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, err
	}
	if err := s.requireRoot(); err != nil {
		return nil, err
	}
	f, err := safeOpenAt(s.rootFD, []string{objectID, filename}, os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("read object file %s/%s: %w", objectID, filename, err)
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (s *Store) DeleteObjectFile(objectID, filename string) error {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return err
	}
	return s.withObjectLock(objectID, func() error {
		if err := s.requireRoot(); err != nil {
			return err
		}
		if err := safeUnlinkAt(s.rootFD, []string{objectID, filename}); err != nil {
			if errors.Is(err, os.ErrNotExist) || isPlatformNotExist(err) {
				return model.ErrNotFound
			}
			return fmt.Errorf("delete object file %s/%s: %w", objectID, filename, err)
		}
		return s.fsyncDirAt([]string{objectID})
	})
}

func (s *Store) ListObjectFolderFiles(objectID string) ([]string, error) {
	if err := objectpath.ValidateObjectID(objectID); err != nil {
		return nil, err
	}
	if err := s.requireRoot(); err != nil {
		return nil, err
	}
	dir, err := safeOpenAt(s.rootFD, []string{objectID}, os.O_RDONLY|openDirectoryFlag, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("list object folder %s: %w", objectID, err)
	}
	defer dir.Close()
	entries, err := dir.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("list object folder %s: %w", objectID, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("invalid path: object file %s resolves through a symlink", entry.Name())
		}
		if !entry.IsDir() && !strings.EqualFold(entry.Name(), objectpath.ManifestFilename) {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func (s *Store) ReadManifestFile(objectID string) ([]byte, error) {
	if err := objectpath.ValidateObjectID(objectID); err != nil {
		return nil, err
	}
	return s.readManifestUnlocked(objectID)
}

func (s *Store) readManifestUnlocked(objectID string) ([]byte, error) {
	if err := s.requireRoot(); err != nil {
		return nil, err
	}
	f, err := safeOpenAt(s.rootFD, []string{objectID, objectpath.ManifestFilename}, os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("read manifest for %s: %w", objectID, err)
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (s *Store) WriteManifestFile(objectID string, data []byte) error {
	if err := objectpath.ValidateObjectID(objectID); err != nil {
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
	if err := objectpath.ValidateObjectID(objectID); err != nil {
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
	if strings.EqualFold(filename, objectpath.ManifestFilename) {
		return fmt.Errorf("invalid path: filename is reserved")
	}
	return nil
}

func (s *Store) GetObjectFileInfo(objectID, filename string) (model.ObjectFileInfo, error) {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return model.ObjectFileInfo{}, err
	}
	if err := s.requireRoot(); err != nil {
		return model.ObjectFileInfo{}, err
	}
	f, err := safeOpenAt(s.rootFD, []string{objectID, filename}, os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ObjectFileInfo{}, model.ErrNotFound
		}
		return model.ObjectFileInfo{}, fmt.Errorf("stat object file %s/%s: %w", objectID, filename, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return model.ObjectFileInfo{}, fmt.Errorf("stat object file %s/%s: %w", objectID, filename, err)
	}
	return model.ObjectFileInfo{Size: info.Size(), UpdatedAt: info.ModTime().UTC()}, nil
}

func (s *Store) ReaderForObjectFile(objectID, filename string) (io.ReadCloser, error) {
	if err := s.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, err
	}
	if err := s.requireRoot(); err != nil {
		return nil, err
	}
	f, err := safeOpenAt(s.rootFD, []string{objectID, filename}, os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("open object file %s/%s: %w", objectID, filename, err)
	}
	return f, nil
}

func (s *Store) writeObjectFileStreamLocked(objectID, filename string, write func(io.Writer) error) error {
	if err := s.requireRoot(); err != nil {
		return err
	}
	tmpName, tmp, err := s.createTempInDir(objectID, ".object-file-write-")
	if err != nil {
		return fmt.Errorf("create temp object file for %s/%s: %w", objectID, filename, err)
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			if unlinkErr := safeUnlinkAt(s.rootFD, []string{objectID, tmpName}); unlinkErr != nil && !errors.Is(unlinkErr, os.ErrNotExist) {
				s.log.Warn("object_storage", "failed to cleanup temp object file", logging.String("object_id", objectID), logging.String("filename", filename), logging.ErrorField(unlinkErr))
			}
		}
	}()
	if err := write(tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp object file %s/%s: %w", objectID, filename, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp object file %s/%s: %w", objectID, filename, err)
	}
	if err := safeRenameAt(s.rootFD, []string{objectID, tmpName}, []string{objectID, filename}); err != nil {
		return fmt.Errorf("rename object file %s/%s: %w", objectID, filename, err)
	}
	cleanupTemp = false
	if err := s.fsyncDirAt([]string{objectID}); err != nil {
		return fmt.Errorf("sync object folder %s after writing %s: %w", objectID, filename, err)
	}
	return nil
}

func (s *Store) writeAppendedObjectFileLocked(objectID, filename string, write func(io.Writer) error) error {
	return s.writeObjectFileStreamLocked(objectID, filename, func(w io.Writer) error {
		reader, err := s.ReaderForObjectFile(objectID, filename)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return err
		}
		if reader != nil {
			defer reader.Close()
			if _, err := io.Copy(w, reader); err != nil {
				return fmt.Errorf("copy existing object file %s/%s: %w", objectID, filename, err)
			}
		}
		return write(w)
	})
}

func (s *Store) currentObjectFileSizeLocked(objectID, filename string) (int64, error) {
	info, err := s.GetObjectFileInfo(objectID, filename)
	if err == nil {
		return info.Size, nil
	}
	if errors.Is(err, model.ErrNotFound) {
		return 0, nil
	}
	return 0, err
}

func (s *Store) requireRoot() error {
	if s.rootFD == nil {
		return fmt.Errorf("object storage root is not initialized")
	}
	return nil
}

func (s *Store) fsyncRoot() error {
	if s.rootFD == nil {
		return nil
	}
	if err := s.rootFD.Sync(); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return nil
}

func (s *Store) fsyncDirAt(parts []string) error {
	dir, err := safeOpenAt(s.rootFD, parts, os.O_RDONLY|openDirectoryFlag, 0)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return nil
}

func (s *Store) writeManifestFileUnlocked(objectID string, data []byte) error {
	if err := s.requireRoot(); err != nil {
		return err
	}
	tmpName, tmp, err := s.createTempInDir(objectID, "."+objectpath.ManifestFilename+".tmp-")
	if err != nil {
		return fmt.Errorf("create manifest temp file for %s: %w", objectID, err)
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = safeUnlinkAt(s.rootFD, []string{objectID, tmpName})
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write manifest temp file for %s: %w", objectID, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync manifest temp file for %s: %w", objectID, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close manifest temp file for %s: %w", objectID, err)
	}
	if err := safeRenameAt(s.rootFD, []string{objectID, tmpName}, []string{objectID, objectpath.ManifestFilename}); err != nil {
		return fmt.Errorf("rename manifest for %s: %w", objectID, err)
	}
	cleanupTemp = false
	if err := s.fsyncDirAt([]string{objectID}); err != nil {
		return fmt.Errorf("sync object folder for %s: %w", objectID, err)
	}
	return nil
}

func (s *Store) createTempInDir(objectID, prefix string) (string, *os.File, error) {
	for i := 0; i < 1000; i++ {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", nil, fmt.Errorf("generate temp suffix: %w", err)
		}
		name := prefix + hex.EncodeToString(b[:])
		f, err := safeOpenAt(s.rootFD, []string{objectID, name}, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", nil, err
		}
		return name, f, nil
	}
	return "", nil, fmt.Errorf("create unique temp file: too many collisions")
}

func (s *Store) withObjectLock(objectID string, fn func() error) error {
	lock := s.acquireObjectLock(objectID)
	lock.mu.Lock()
	defer func() {
		lock.mu.Unlock()
		s.releaseObjectLock(objectID, lock)
	}()
	return fn()
}

func (s *Store) acquireObjectLock(objectID string) *objectLock {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()

	lock := s.locks[objectID]
	if lock == nil {
		lock = &objectLock{}
		s.locks[objectID] = lock
	}
	lock.refs++
	return lock
}

func (s *Store) releaseObjectLock(objectID string, lock *objectLock) {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()

	lock.refs--
	if lock.refs == 0 {
		delete(s.locks, objectID)
	}
}

type objectLock struct {
	mu   sync.Mutex
	refs int
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
