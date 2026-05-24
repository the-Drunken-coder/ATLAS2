//go:build unix

package objectstorage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const openDirectoryFlag = unix.O_DIRECTORY

func openRootPathNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|openDirectoryFlag, 0)
}

func isPlatformNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT)
}

// safeOpenAt opens the file or directory identified by parts (a slice of
// individual path components) under root, refusing to follow symlinks at any
// depth. Each intermediate component is opened with O_RDONLY|O_DIRECTORY|
// O_NOFOLLOW, then the leaf is opened with the caller-provided flags (with
// O_NOFOLLOW forced on). This closes the TOCTOU window that a
// path-string-then-Lstat-then-open approach has, because every step is rooted
// at a directory FD that the kernel validates atomically.
//
// All resolved components must live on the same filesystem device as root;
// crossing a mount or device boundary is rejected.
func safeOpenAt(root *os.File, parts []string, flags int, mode os.FileMode) (*os.File, error) {
	if root == nil {
		return nil, fmt.Errorf("safeOpenAt: nil root")
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, `/\`) {
			return nil, fmt.Errorf("safeOpenAt: invalid path component %q", p)
		}
	}

	rootStat, err := statFD(int(root.Fd()))
	if err != nil {
		return nil, fmt.Errorf("safeOpenAt: stat root: %w", err)
	}

	dirFD := int(root.Fd())
	closeDir := func() {}

	for i := 0; i < len(parts)-1; i++ {
		nextFD, err := unix.Openat(dirFD, parts[i], unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		if err != nil {
			closeDir()
			return nil, mapOpenatErr(err)
		}
		// Verify the resolved component is still on the same device as the
		// storage root — refuse to cross filesystem boundaries.
		if nst, statErr := statFD(nextFD); statErr != nil {
			_ = unix.Close(nextFD)
			closeDir()
			return nil, fmt.Errorf("safeOpenAt: stat component %q: %w", parts[i], statErr)
		} else if nst.Dev != rootStat.Dev {
			_ = unix.Close(nextFD)
			closeDir()
			return nil, fmt.Errorf("safeOpenAt: component %q crosses filesystem boundary", parts[i])
		}
		closeDir()
		fd := nextFD
		dirFD = fd
		closeDir = func() { _ = unix.Close(fd) }
	}

	leafName := ""
	if n := len(parts); n > 0 {
		leafName = parts[n-1]
	}
	if leafName == "" {
		// Caller asked for root itself — return an owned FD so defer Close does not drop the shared root.
		closeDir()
		leafFD, err := unix.Openat(dirFD, ".", unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		if err != nil {
			return nil, mapOpenatErr(err)
		}
		if lst, statErr := statFD(leafFD); statErr != nil {
			_ = unix.Close(leafFD)
			return nil, fmt.Errorf("safeOpenAt: stat root handle: %w", statErr)
		} else if lst.Dev != rootStat.Dev {
			_ = unix.Close(leafFD)
			return nil, fmt.Errorf("safeOpenAt: root handle crosses filesystem boundary")
		}
		return os.NewFile(uintptr(leafFD), root.Name()), nil
	}

	leafFD, err := unix.Openat(dirFD, leafName, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode))
	closeDir()
	if err != nil {
		return nil, mapOpenatErr(err)
	}
	if lst, statErr := statFD(leafFD); statErr != nil {
		_ = unix.Close(leafFD)
		return nil, fmt.Errorf("safeOpenAt: stat leaf: %w", statErr)
	} else if lst.Dev != rootStat.Dev {
		_ = unix.Close(leafFD)
		return nil, fmt.Errorf("safeOpenAt: leaf %q crosses filesystem boundary", leafName)
	}

	return os.NewFile(uintptr(leafFD), filepath.Join(append([]string{root.Name()}, parts...)...)), nil
}

// safeMkdirAt creates a directory under root via unix.Mkdirat, walking parents
// with O_PATH|O_NOFOLLOW so no symlink at any depth is followed.
func safeMkdirAt(root *os.File, parts []string, mode os.FileMode) error {
	if len(parts) == 0 {
		return fmt.Errorf("safeMkdirAt: empty parts")
	}
	parentFD, leaf, closeFn, err := walkParents(root, parts)
	if err != nil {
		return err
	}
	defer closeFn()
	if err := unix.Mkdirat(parentFD, leaf, uint32(mode.Perm())); err != nil {
		if errors.Is(err, syscall.EEXIST) {
			return os.ErrExist
		}
		return &fs.PathError{Op: "mkdirat", Path: leaf, Err: err}
	}
	return nil
}

// safeUnlinkAt removes a file (not a directory) under root via unix.Unlinkat.
func safeUnlinkAt(root *os.File, parts []string) error {
	if len(parts) == 0 {
		return fmt.Errorf("safeUnlinkAt: empty parts")
	}
	parentFD, leaf, closeFn, err := walkParents(root, parts)
	if err != nil {
		return err
	}
	defer closeFn()
	if err := unix.Unlinkat(parentFD, leaf, 0); err != nil {
		return &fs.PathError{Op: "unlinkat", Path: leaf, Err: err}
	}
	return nil
}

// safeRenameAt renames oldParts -> newParts under root via unix.Renameat. Both
// paths must resolve under the same root and are validated component-wise.
func safeRenameAt(root *os.File, oldParts, newParts []string) error {
	if len(oldParts) == 0 || len(newParts) == 0 {
		return fmt.Errorf("safeRenameAt: empty parts")
	}
	oldParentFD, oldLeaf, closeOld, err := walkParents(root, oldParts)
	if err != nil {
		return err
	}
	defer closeOld()
	newParentFD, newLeaf, closeNew, err := walkParents(root, newParts)
	if err != nil {
		return err
	}
	defer closeNew()
	if err := unix.Renameat(oldParentFD, oldLeaf, newParentFD, newLeaf); err != nil {
		return &fs.PathError{Op: "renameat", Path: oldLeaf, Err: err}
	}
	return nil
}

func safeRemoveAllAt(root *os.File, parts []string) error {
	if len(parts) == 0 {
		return fmt.Errorf("safeRemoveAllAt: empty parts")
	}
	parentFD, leaf, closeFn, err := walkParents(root, parts)
	if err != nil {
		return err
	}
	defer closeFn()
	return removeAllAt(parentFD, leaf)
}

func walkParents(root *os.File, parts []string) (int, string, func(), error) {
	if root == nil {
		return -1, "", func() {}, fmt.Errorf("walkParents: nil root")
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, `/\`) {
			return -1, "", func() {}, fmt.Errorf("walkParents: invalid path component %q", p)
		}
	}
	rootStat, err := statFD(int(root.Fd()))
	if err != nil {
		return -1, "", func() {}, fmt.Errorf("walkParents: stat root: %w", err)
	}

	dirFD := int(root.Fd())
	owned := false
	closeFn := func() {}
	for i := 0; i < len(parts)-1; i++ {
		nextFD, err := unix.Openat(dirFD, parts[i], unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		if err != nil {
			closeFn()
			return -1, "", func() {}, mapOpenatErr(err)
		}
		if nst, statErr := statFD(nextFD); statErr != nil {
			_ = unix.Close(nextFD)
			closeFn()
			return -1, "", func() {}, fmt.Errorf("walkParents: stat component %q: %w", parts[i], statErr)
		} else if nst.Dev != rootStat.Dev {
			_ = unix.Close(nextFD)
			closeFn()
			return -1, "", func() {}, fmt.Errorf("walkParents: component %q crosses filesystem boundary", parts[i])
		}
		if owned {
			_ = unix.Close(dirFD)
		}
		fd := nextFD
		dirFD = fd
		owned = true
		closeFn = func() { _ = unix.Close(fd) }
	}
	return dirFD, parts[len(parts)-1], closeFn, nil
}

func removeAllAt(parentFD int, name string) error {
	if err := unix.Unlinkat(parentFD, name, 0); err == nil || errors.Is(err, unix.ENOENT) {
		return nil
	} else if !errors.Is(err, unix.EISDIR) && !errors.Is(err, unix.EPERM) {
		return &fs.PathError{Op: "unlinkat", Path: name, Err: err}
	}

	childFD, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return mapOpenatErr(err)
	}
	child := os.NewFile(uintptr(childFD), name)
	entries, readErr := child.Readdirnames(-1)
	if readErr != nil {
		_ = child.Close()
		return fmt.Errorf("safeRemoveAllAt: read directory %q: %w", name, readErr)
	}
	for _, entry := range entries {
		if err := removeAllAt(childFD, entry); err != nil {
			_ = child.Close()
			return err
		}
	}
	if err := child.Close(); err != nil {
		return fmt.Errorf("safeRemoveAllAt: close directory %q: %w", name, err)
	}
	if err := unix.Unlinkat(parentFD, name, unix.AT_REMOVEDIR); err != nil && !errors.Is(err, unix.ENOENT) {
		return &fs.PathError{Op: "unlinkat", Path: name, Err: err}
	}
	return nil
}

func statFD(fd int) (*syscall.Stat_t, error) {
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func mapOpenatErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.EMLINK) {
		return fmt.Errorf("invalid path: refused to follow symlink (%w)", err)
	}
	if errors.Is(err, unix.ENOENT) {
		return os.ErrNotExist
	}
	if errors.Is(err, unix.EEXIST) {
		return os.ErrExist
	}
	return err
}
