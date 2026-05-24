package runtime

import (
	"testing"
	"time"
)

func TestDecodeCheckpointMigratesLegacyObservedAt(t *testing.T) {
	legacyAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	checkpoint, err := decodeCheckpoint([]byte(`{
		"observation_id": "obs_legacy",
		"version": 3,
		"observed_at": "2026-01-02T03:04:05Z"
	}`))
	if err != nil {
		t.Fatalf("decodeCheckpoint failed: %v", err)
	}
	if checkpoint.ObservationID != "obs_legacy" || checkpoint.Version != 3 {
		t.Fatalf("unexpected checkpoint identity: %+v", checkpoint)
	}
	if !checkpoint.UpdatedAt.Equal(legacyAt) {
		t.Fatalf("expected updated_at %v, got %v", legacyAt, checkpoint.UpdatedAt)
	}
}

func TestDecodeCheckpointPrefersUpdatedAt(t *testing.T) {
	updatedAt := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	legacyAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	checkpoint, err := decodeCheckpoint([]byte(`{
		"observation_id": "obs_current",
		"version": 2,
		"updated_at": "2026-02-03T04:05:06Z",
		"observed_at": "2026-01-02T03:04:05Z"
	}`))
	if err != nil {
		t.Fatalf("decodeCheckpoint failed: %v", err)
	}
	if !checkpoint.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected updated_at %v, got %v", updatedAt, checkpoint.UpdatedAt)
	}
	if checkpoint.UpdatedAt.Equal(legacyAt) {
		t.Fatal("expected legacy observed_at to be ignored when updated_at is set")
	}
}
