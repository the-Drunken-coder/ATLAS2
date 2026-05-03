//go:build !unix

package objectstorage

import (
	"fmt"
	"os"
)

func openFileNoFollow(string, int, os.FileMode) (*os.File, error) {
	return nil, fmt.Errorf("secure no-follow file open is unsupported on this platform")
}
