package model

import "time"

type EntityType string

const (
	EntityTypeAsset       EntityType = "asset"
	EntityTypeTrack       EntityType = "track"
	EntityTypeGeofeature  EntityType = "geofeature"
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
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Object struct {
	ObjectID  string    `json:"object_id"`
	Type      string    `json:"type"`
	OwnerType OwnerType `json:"owner_type"`
	OwnerID   string    `json:"owner_id"`
	JSON      []byte    `json:"json"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Task struct {
	TaskID                  string     `json:"task_id"`
	Status                  TaskStatus `json:"status"`
	AssetID                 string     `json:"asset_id"`
	CommandCatalogObjectID  string     `json:"command_catalog_object_id"`
	JSON                    []byte     `json:"json"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type Observation struct {
	ObservationID string    `json:"observation_id"`
	SourceAssetID string    `json:"source_asset_id"`
	JSON          []byte    `json:"json"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ObjectManifest struct {
	Files map[string]ObjectFileInfo `json:"files"`
}

type ObjectFileInfo struct {
	Size      int64  `json:"size"`
	UpdatedAt string `json:"updated_at"`
}
