//go:build !unix

package objectstorage

import (
	"fmt"
	"os"
)

const openDirectoryFlag = 0

func openRootPathNoFollow(path string) (*os.File, error) {
	return nil, fmt.Errorf("secure no-follow directory open is unsupported on this platform")
}

func isPlatformNotExist(err error) bool {
	return os.IsNotExist(err)
}

func openFileNoFollow(string, int, os.FileMode) (*os.File, error) {
	return nil, fmt.Errorf("secure no-follow file open is unsupported on this platform")
}

func safeOpenAt(*os.File, []string, int, os.FileMode) (*os.File, error) {
	return nil, fmt.Errorf("secure openat is unsupported on this platform")
}

func safeMkdirAt(*os.File, []string, os.FileMode) error {
	return fmt.Errorf("secure mkdirat is unsupported on this platform")
}

func safeUnlinkAt(*os.File, []string) error {
	return fmt.Errorf("secure unlinkat is unsupported on this platform")
}

func safeRenameAt(*os.File, []string, []string) error {
	return fmt.Errorf("secure renameat is unsupported on this platform")
}

func safeRemoveAllAt(*os.File, []string) error {
	return fmt.Errorf("secure remove-all is unsupported on this platform")
}
