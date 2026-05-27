package sim

import (
	"fmt"
	"time"
)

// Run executes a simulation definition and returns observation stream snapshots plus final ground truth.
func Run(def Definition) (Result, error) {
	if def.Start.IsZero() {
		if def.StartRFC3339 == "" {
			return Result{}, fmt.Errorf("simulation start time is required")
		}
		start, err := time.Parse(time.RFC3339Nano, def.StartRFC3339)
		if err != nil {
			start, err = time.Parse(time.RFC3339, def.StartRFC3339)
			if err != nil {
				return Result{}, fmt.Errorf("parse start: %w", err)
			}
		}
		def.Start = start.UTC()
	}
	if def.DurationSec <= 0 {
		return Result{}, fmt.Errorf("duration_sec must be positive")
	}
	if len(def.Feeds) == 0 {
		return Result{}, fmt.Errorf("at least one feed is required")
	}

	duration := time.Duration(def.DurationSec * float64(time.Second))
	rng := newRNG(def.Seed)
	var observations []ObservationSnapshot

	for _, feedCfg := range def.Feeds {
		snap, err := runFeed(feedCfg, rng, def.Start, duration, def.Target)
		if err != nil {
			return Result{}, err
		}
		observations = append(observations, snap)
	}

	name := def.Name
	if name == "" {
		name = "simulation"
	}
	return Result{
		Name:         name,
		Observations: observations,
		GroundTruth:  def.Target.StateAt(def.Start, duration),
	}, nil
}

func runFeed(cfg FeedConfig, rng *rng, start time.Time, duration time.Duration, motion Motion) (ObservationSnapshot, error) {
	switch cfg.Type {
	case "adsb", "ads-b":
		feed, err := newADSBFeed(cfg, rng)
		if err != nil {
			return ObservationSnapshot{}, err
		}
		return feed.run(start, duration, motion)
	case "camera_lob", "camera":
		feed, err := newCameraLOBFeed(cfg, rng)
		if err != nil {
			return ObservationSnapshot{}, err
		}
		return feed.run(start, duration, motion)
	default:
		return ObservationSnapshot{}, fmt.Errorf("unknown feed type %q", cfg.Type)
	}
}
