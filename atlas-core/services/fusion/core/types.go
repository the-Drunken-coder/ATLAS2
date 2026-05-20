package core

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

type Checkpoint struct {
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	ObservationID string    `json:"observation_id,omitempty"`
	Version       int       `json:"version,omitempty"`
	EngineName    string    `json:"engine_name,omitempty"`
	EngineVersion string    `json:"engine_version,omitempty"`
	ObservedAt    time.Time `json:"observed_at,omitempty"` // Deprecated: use UpdatedAt
}

func (c Checkpoint) IsZero() bool {
	return c.UpdatedAt.IsZero() && c.ObservationID == ""
}

type ObservationQuery struct {
	Checkpoint Checkpoint
	PageSize   int32
}

type ObservationInput struct {
	ObservationID     string          `json:"observation_id"`
	SourceAssetID     string          `json:"source_asset_id"`
	TargetEntityID    *string         `json:"target_entity_id,omitempty"`
	LatestTelemetryAt time.Time       `json:"latest_telemetry_at"`
	Version           int             `json:"version"`
	UpdatedAt         time.Time       `json:"updated_at"`
	JSON              json.RawMessage `json:"json"`
}

func (o ObservationInput) Ref() InputRef {
	return InputRef{
		ObservationID: o.ObservationID,
		Version:       o.Version,
		ObservedAt:    o.LatestTelemetryAt.UTC(),
	}
}

type InputRef struct {
	ObservationID string    `json:"observation_id"`
	Version       int       `json:"version"`
	ObservedAt    time.Time `json:"observed_at"`
}

type ObservationBatch struct {
	Observations   []ObservationInput `json:"observations"`
	NextCheckpoint Checkpoint         `json:"next_checkpoint"`
}

func NewObservationBatch(observations []ObservationInput, current Checkpoint) ObservationBatch {
	ordered := append([]ObservationInput(nil), observations...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].UpdatedAt.Equal(ordered[j].UpdatedAt) {
			if ordered[i].ObservationID == ordered[j].ObservationID {
				return ordered[i].Version < ordered[j].Version
			}
			return ordered[i].ObservationID < ordered[j].ObservationID
		}
		return ordered[i].UpdatedAt.Before(ordered[j].UpdatedAt)
	})
	return ObservationBatch{
		Observations:   ordered,
		NextCheckpoint: NextCheckpoint(ordered, current),
	}
}

func NextCheckpoint(observations []ObservationInput, current Checkpoint) Checkpoint {
	next := current
	for _, obs := range observations {
		updatedAt := obs.UpdatedAt.UTC()
		if updatedAt.After(next.UpdatedAt) ||
			(updatedAt.Equal(next.UpdatedAt) && obs.ObservationID > next.ObservationID) ||
			(updatedAt.Equal(next.UpdatedAt) && obs.ObservationID == next.ObservationID && obs.Version > next.Version) {
			next.UpdatedAt = updatedAt
			next.ObservationID = obs.ObservationID
			next.Version = obs.Version
		}
	}
	return next
}

func AfterCheckpoint(obs ObservationInput, checkpoint Checkpoint) bool {
	if checkpoint.IsZero() {
		return true
	}
	updatedAt := obs.UpdatedAt.UTC()
	if updatedAt.After(checkpoint.UpdatedAt) {
		return true
	}
	if updatedAt.Equal(checkpoint.UpdatedAt) && obs.ObservationID > checkpoint.ObservationID {
		return true
	}
	return updatedAt.Equal(checkpoint.UpdatedAt) && obs.ObservationID == checkpoint.ObservationID && obs.Version > checkpoint.Version
}

type TrackUpdate struct {
	TrackID            string          `json:"track_id"`
	JSON               json.RawMessage `json:"json"`
	ProvenanceObjectID string          `json:"provenance_object_id,omitempty"`
}

type ProvenanceRecord struct {
	TrackID       string          `json:"track_id"`
	EngineName    string          `json:"engine_name"`
	EngineVersion string          `json:"engine_version,omitempty"`
	Inputs        []InputRef      `json:"inputs"`
	JSON          json.RawMessage `json:"json,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type Result struct {
	TrackUpdates []TrackUpdate      `json:"track_updates"`
	Provenance   []ProvenanceRecord `json:"provenance"`
}

type Source interface {
	Fetch(ctx context.Context, query ObservationQuery) (ObservationBatch, error)
}

type Engine interface {
	Name() string
	Version() string
	Fuse(ctx context.Context, batch ObservationBatch) (Result, error)
}

type Sink interface {
	Commit(ctx context.Context, result Result) error
}

type CheckpointStore interface {
	Load(ctx context.Context) (Checkpoint, error)
	Save(ctx context.Context, checkpoint Checkpoint) error
}
