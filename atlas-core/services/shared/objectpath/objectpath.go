package objectpath

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ManifestFilename is the reserved manifest file inside each object folder.
const ManifestFilename = "manifest.json"

var objectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ValidateObjectID checks that an object ID is safe for filesystem-backed storage.
func ValidateObjectID(objectID string) error {
	if objectID == "" {
		return fmt.Errorf("object_id is required")
	}
	if objectID == "." || objectID == ".." {
		return fmt.Errorf("invalid path: object_id must not be '.' or '..'")
	}
	if objectID == ManifestFilename {
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
