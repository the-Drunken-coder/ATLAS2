package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
)

type Runner struct {
	Source          core.Source
	Engines         []core.Engine
	Sink            core.Sink
	CheckpointStore core.CheckpointStore
	PageSize        int32
}

type RunStats struct {
	ObservationCount int
	TrackUpdateCount int
	ProvenanceCount  int
	Checkpoint       core.Checkpoint
}

func (r Runner) RunOnce(ctx context.Context) (RunStats, error) {
	if r.Source == nil {
		return RunStats{}, fmt.Errorf("fusion source is required")
	}
	if len(r.Engines) == 0 {
		return RunStats{}, fmt.Errorf("at least one fusion engine is required")
	}
	if r.Sink == nil {
		return RunStats{}, fmt.Errorf("fusion sink is required")
	}

	checkpoint := core.Checkpoint{}
	var err error
	if r.CheckpointStore != nil {
		checkpoint, err = r.CheckpointStore.Load(ctx)
		if err != nil {
			return RunStats{}, err
		}
	}

	batch, err := r.Source.Fetch(ctx, core.ObservationQuery{
		Checkpoint: checkpoint,
		PageSize:   r.PageSize,
	})
	if err != nil {
		return RunStats{}, err
	}
	if len(batch.Observations) == 0 {
		return RunStats{Checkpoint: checkpoint}, nil
	}

	stats := RunStats{ObservationCount: len(batch.Observations)}
	for _, engine := range r.Engines {
		result, err := engine.Fuse(ctx, batch)
		if err != nil {
			return stats, fmt.Errorf("run engine %q: %w", engine.Name(), err)
		}
		for i := range result.Provenance {
			if result.Provenance[i].EngineName == "" {
				result.Provenance[i].EngineName = engine.Name()
			}
			if result.Provenance[i].EngineVersion == "" {
				result.Provenance[i].EngineVersion = engine.Version()
			}
			if result.Provenance[i].CreatedAt.IsZero() {
				result.Provenance[i].CreatedAt = time.Now().UTC()
			}
		}
		if err := r.Sink.Commit(ctx, result); err != nil {
			return stats, fmt.Errorf("commit engine %q result: %w", engine.Name(), err)
		}
		stats.TrackUpdateCount += len(result.TrackUpdates)
		stats.ProvenanceCount += len(result.Provenance)
	}

	next := batch.NextCheckpoint
	if len(r.Engines) == 1 {
		next.EngineName = r.Engines[0].Name()
		next.EngineVersion = r.Engines[0].Version()
	} else {
		next.EngineName = "multi"
		next.EngineVersion = ""
	}
	if r.CheckpointStore != nil {
		if err := r.CheckpointStore.Save(ctx, next); err != nil {
			return stats, err
		}
	}
	stats.Checkpoint = next
	return stats, nil
}
