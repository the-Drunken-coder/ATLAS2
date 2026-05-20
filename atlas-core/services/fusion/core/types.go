package core

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

type Checkpoint struct {
	ObservedAt    time.Time `json:"observed_at,omitempty"`
	ObservationID string    `json:"observation_id,omitempty"`
	EngineName    string    `json:"engine_name,omitempty"`
	EngineVersion string    `json:"engine_version,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

func (c Checkpoint) IsZero() bool {
	return c.ObservedAt.IsZero() && c.ObservationID == ""
}

type ObservationQuery struct {
	Checkpoint Checkpoint
	PageSize   int32
}

type ObservationInput struct {
	ObservationID  string          `json:"observation_id"`
	SourceAssetID  string          `json:"source_asset_id"`
	TargetEntityID *string         `json:"target_entity_id,omitempty"`
	ObservedAt     time.Time       `json:"observed_at"`
	Version        int             `json:"version"`
	JSON           json.RawMessage `json:"json"`
}

func (o ObservationInput) Ref() InputRef {
	return InputRef{
		ObservationID: o.ObservationID,
		Version:       o.Version,
		ObservedAt:    o.ObservedAt.UTC(),
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
		if ordered[i].ObservedAt.Equal(ordered[j].ObservedAt) {
			return ordered[i].ObservationID < ordered[j].ObservationID
		}
		return ordered[i].ObservedAt.Before(ordered[j].ObservedAt)
	})
	return ObservationBatch{
		Observations:   ordered,
		NextCheckpoint: NextCheckpoint(ordered, current),
	}
}

func NextCheckpoint(observations []ObservationInput, current Checkpoint) Checkpoint {
	next := current
	for _, obs := range observations {
		if obs.ObservedAt.After(next.ObservedAt) ||
			(obs.ObservedAt.Equal(next.ObservedAt) && obs.ObservationID > next.ObservationID) {
			next.ObservedAt = obs.ObservedAt.UTC()
			next.ObservationID = obs.ObservationID
		}
	}
	return next
}

func AfterCheckpoint(obs ObservationInput, checkpoint Checkpoint) bool {
	if checkpoint.IsZero() {
		return true
	}
	if obs.ObservedAt.After(checkpoint.ObservedAt) {
		return true
	}
	return obs.ObservedAt.Equal(checkpoint.ObservedAt) && obs.ObservationID > checkpoint.ObservationID
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
