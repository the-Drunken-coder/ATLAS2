//go:build unix

package objectstorage

import (
	"os"
	"syscall"
)

func openFileNoFollow(path string, flags int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flags|syscall.O_NOFOLLOW, perm)
}
