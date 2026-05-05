package model

import (
	"crypto/sha256"
	"encoding/hex"
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
	ObjectTypeCommandCatalog ObjectType = "command_catalog"
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
	return []ObjectType{ObjectTypeCommandCatalog, ObjectTypeLog, ObjectTypePhoto}
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
