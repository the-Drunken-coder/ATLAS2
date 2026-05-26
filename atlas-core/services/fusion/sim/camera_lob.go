package sim

import (
	"encoding/json"
	"fmt"
	"time"
)

// cameraLOBFeed simulates a fixed camera reporting observer pose plus bearing/elevation to the target.
type cameraLOBFeed struct {
	cfg FeedConfig
	rng *rng
}

func newCameraLOBFeed(cfg FeedConfig, rng *rng) (*cameraLOBFeed, error) {
	if cfg.IntervalMS <= 0 {
		cfg.IntervalMS = 1000
	}
	cfg.DelayMSMin = max(0, cfg.DelayMSMin)
	cfg.DelayMSMax = max(0, cfg.DelayMSMax)
	if cfg.DelayMSMax < cfg.DelayMSMin {
		cfg.DelayMSMax = cfg.DelayMSMin
	}
	return &cameraLOBFeed{cfg: cfg, rng: rng}, nil
}

func (f *cameraLOBFeed) run(start time.Time, duration time.Duration, motion Motion) (ObservationSnapshot, error) {
	interval := time.Duration(f.cfg.IntervalMS) * time.Millisecond
	var snap ObservationSnapshot
	snap.ObservationID = f.cfg.ObservationID
	snap.SourceAssetID = f.cfg.SourceAssetID

	for at := time.Duration(0); at <= duration; at += interval {
		if f.rng.chance(f.cfg.DropProbability) {
			snap.SamplesDropped++
			continue
		}
		observedAt := start.Add(at)
		target := motion.StateAt(start, at)
		azimuth := BearingDegrees(f.cfg.ObserverLat, f.cfg.ObserverLon, target.Latitude, target.Longitude)
		elevation := ElevationDegrees(f.cfg.ObserverLat, f.cfg.ObserverLon, f.cfg.ObserverAltM, target.Latitude, target.Longitude, target.AltitudeM)
		azimuth += f.rng.uniform(-f.cfg.BearingNoiseDeg, f.cfg.BearingNoiseDeg)
		elevation += f.rng.uniform(-f.cfg.ElevationNoiseDeg, f.cfg.ElevationNoiseDeg)
		if azimuth < 0 {
			azimuth += 360
		}
		if azimuth >= 360 {
			azimuth -= 360
		}

		receivedAt := observedAt.Add(time.Duration(f.rng.uniform(float64(f.cfg.DelayMSMin), float64(f.cfg.DelayMSMax)+1)) * time.Millisecond)
		jsonBytes, err := f.lobTelemetryJSON(observedAt, azimuth, elevation)
		if err != nil {
			return ObservationSnapshot{}, err
		}
		snap.LatestTelemetryAt = observedAt
		snap.UpdatedAt = receivedAt
		snap.Version++
		snap.JSON = jsonBytes
		snap.SamplesEmitted++
	}

	if snap.SamplesEmitted == 0 {
		return ObservationSnapshot{}, fmt.Errorf("camera_lob feed %q produced no samples", f.cfg.ObservationID)
	}
	return snap, nil
}

func (f *cameraLOBFeed) lobTelemetryJSON(observedAt time.Time, azimuth, elevation float64) ([]byte, error) {
	payload := map[string]any{
		"latest_telemetry": map[string]any{
			"observed_at": observedAt.UTC().Format(time.RFC3339Nano),
			"kind":        "line_of_bearing",
			"data": map[string]any{
				"observer_latitude":  f.cfg.ObserverLat,
				"observer_longitude": f.cfg.ObserverLon,
				"observer_altitude_m": f.cfg.ObserverAltM,
				"azimuth_deg":        azimuth,
				"elevation_deg":      elevation,
			},
		},
	}
	return json.Marshal(payload)
}
