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

// ValidateObjectID checks IDs used for create, rename, and file I/O (strict charset and reserved names).
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

// ValidateDeletableFolderName checks root-level folder names for delete/reconcile cleanup only.
// It is looser than ValidateObjectID so legacy on-disk folders can still be removed.
func ValidateDeletableFolderName(name string) error {
	if name == "" {
		return fmt.Errorf("object_id is required")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid path: object_id must not be '.' or '..'")
	}
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("invalid path: object_id contains path separators")
	}
	return nil
}
