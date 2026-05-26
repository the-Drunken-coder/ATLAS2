package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/fusion/engines"
)

func TestRunnerRunsEngineCommitsTracksAndSavesCheckpoint(t *testing.T) {
	latestTelemetryAt := time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)
	source := fakeSource{batch: core.NewObservationBatch([]core.ObservationInput{{
		ObservationID:     "obs_001",
		SourceAssetID:     "asset_camera",
		LatestTelemetryAt: latestTelemetryAt,
		UpdatedAt:         updatedAt,
		Version:           1,
		JSON:              json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T00:10:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0,"uncertainty_radius_m":25}}}`),
	}}, core.Checkpoint{})}
	sink := &captureSink{}
	checkpoints := &memoryCheckpointStore{}

	stats, err := (Runner{
		Source:          source,
		Engines:         mustResolveEngines(t, []string{"reference"}),
		Sink:            sink,
		CheckpointStore: checkpoints,
		PageSize:        100,
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if stats.ObservationCount != 1 || stats.TrackUpdateCount != 1 || stats.ProvenanceCount != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(sink.results) != 1 || len(sink.results[0].TrackUpdates) != 1 {
		t.Fatalf("expected one committed track update, got %+v", sink.results)
	}
	if checkpoints.saved.ObservationID != "obs_001" || !checkpoints.saved.UpdatedAt.Equal(updatedAt) || checkpoints.saved.Version != 1 {
		t.Fatalf("unexpected saved checkpoint: %+v", checkpoints.saved)
	}
	if checkpoints.saved.EngineName != "reference" || checkpoints.saved.EngineVersion != "v1" {
		t.Fatalf("expected engine checkpoint metadata, got %+v", checkpoints.saved)
	}
}

type fakeSource struct {
	batch core.ObservationBatch
}

func (s fakeSource) Fetch(context.Context, core.ObservationQuery) (core.ObservationBatch, error) {
	return s.batch, nil
}

type captureSink struct {
	results []core.Result
}

func (s *captureSink) Commit(_ context.Context, result core.Result) error {
	s.results = append(s.results, result)
	return nil
}

type memoryCheckpointStore struct {
	loaded core.Checkpoint
	saved  core.Checkpoint
}

func (s *memoryCheckpointStore) Load(context.Context) (core.Checkpoint, error) {
	return s.loaded, nil
}

func (s *memoryCheckpointStore) Save(_ context.Context, checkpoint core.Checkpoint) error {
	s.saved = checkpoint
	return nil
}

func mustResolveEngines(t *testing.T, names []string) []core.Engine {
	t.Helper()
	resolved, err := engines.Resolve(names)
	if err != nil {
		t.Fatalf("Resolve engines: %v", err)
	}
	return resolved
}
