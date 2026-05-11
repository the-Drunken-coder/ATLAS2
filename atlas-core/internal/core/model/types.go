package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type EntityType string

const (
	EntityTypeAsset      EntityType = "asset"
	EntityTypeTrack      EntityType = "track"
	EntityTypeGeofeature EntityType = "geofeature"
)

type ObjectType string

const (
	ObjectTypeDocument ObjectType = "document"
	// Deprecated: use ObjectTypeDocument. Vertical Slice 2 stores the command
	// catalog as a document object with object_id = command_catalog.
	ObjectTypeCommandCatalog ObjectType = ObjectTypeDocument
	ObjectTypeLog            ObjectType = "log"
	ObjectTypePhoto          ObjectType = "photo"
)

type OwnerType string

const (
	OwnerTypeEntity      OwnerType = "entity"
	OwnerTypeObservation OwnerType = "observation"
	OwnerTypeTask        OwnerType = "task"
	OwnerTypeSystem      OwnerType = "system"
)

type TaskStatus string

const (
	TaskStatusPending      TaskStatus = "pending"
	TaskStatusAcknowledged TaskStatus = "acknowledged"
	TaskStatusCompleted    TaskStatus = "completed"
	TaskStatusFailed       TaskStatus = "failed"
)

type Entity struct {
	EntityID  string     `json:"entity_id"`
	Type      EntityType `json:"type"`
	Subtype   *string    `json:"subtype,omitempty"`
	Alias     *string    `json:"alias,omitempty"`
	JSON      []byte     `json:"json"`
	Version   int        `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Object struct {
	ObjectID  string     `json:"object_id"`
	Type      ObjectType `json:"type"`
	OwnerType OwnerType  `json:"owner_type"`
	OwnerID   string     `json:"owner_id"`
	JSON      []byte     `json:"json"`
	Version   int        `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Task struct {
	TaskID                 string     `json:"task_id"`
	Status                 TaskStatus `json:"status"`
	AssetID                string     `json:"asset_id"`
	CommandCatalogObjectID string     `json:"command_catalog_object_id"`
	JSON                   []byte     `json:"json"`
	Version                int        `json:"version"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type Observation struct {
	ObservationID string    `json:"observation_id"`
	SourceAssetID string    `json:"source_asset_id"`
	JSON          []byte    `json:"json"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ObjectManifest struct {
	Version string                    `json:"version,omitempty"`
	Files   map[string]ObjectFileInfo `json:"files"`
}

type ObjectFileInfo struct {
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

func KnownObjectTypes() []ObjectType {
	return []ObjectType{ObjectTypeDocument, ObjectTypeLog, ObjectTypePhoto}
}

func NormalizeManifest(manifest *ObjectManifest) *ObjectManifest {
	if manifest == nil {
		return nil
	}
	if manifest.Files == nil {
		manifest.Files = map[string]ObjectFileInfo{}
	}
	for name, info := range manifest.Files {
		info.UpdatedAt = info.UpdatedAt.UTC()
		manifest.Files[name] = info
	}
	manifest.Version = ManifestVersion(manifest)
	return manifest
}

func ManifestVersion(manifest *ObjectManifest) string {
	if manifest == nil {
		return ""
	}
	keys := make([]string, 0, len(manifest.Files))
	for name := range manifest.Files {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	hash := sha256.New()
	for _, name := range keys {
		info := manifest.Files[name]
		hash.Write([]byte(name))
		hash.Write([]byte{'|'})
		hash.Write([]byte(strings.TrimSpace(info.UpdatedAt.UTC().Format(time.RFC3339Nano))))
		hash.Write([]byte{'|'})
		hash.Write([]byte(strings.TrimSpace(fmtInt64(info.Size))))
		hash.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func fmtInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}

var objectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ValidateObjectID validates that an object ID is safe for filesystem use.
func ValidateObjectID(objectID string) error {
	if objectID == "" {
		return fmt.Errorf("object_id is required")
	}
	if objectID == "." || objectID == ".." {
		return fmt.Errorf("invalid path: object_id must not be '.' or '..'")
	}
	if objectID == "manifest.json" {
		return fmt.Errorf("invalid path: object_id is reserved")
	}
	if filepath.IsAbs(objectID) || strings.ContainsAny(objectID, `/\`) {
		return fmt.Errorf("invalid path: object_id contains path separators")
	}
	if !objectIDPattern.MatchString(objectID) {
		return fmt.Errorf("invalid path: object_id must use only letters, numbers, '_' or '-'")
	}
	return nil
}

// ValidateObjectFilename validates that an object-local filename is safe for filesystem use.
func ValidateObjectFilename(filename string) error {
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
	if strings.ContainsAny(filename, `/\`) {
		return fmt.Errorf("invalid path: filename contains path separators")
	}
	if filename == "manifest.json" {
		return fmt.Errorf("invalid path: filename is reserved")
	}
	return nil
}
