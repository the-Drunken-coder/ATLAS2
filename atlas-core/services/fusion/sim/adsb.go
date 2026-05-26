package sim

import (
	"encoding/json"
	"fmt"
	"time"
)

// adsbFeed simulates 1090ES-style position reports: ~2 Hz samples with delivery delay,
// optional drops, and optional omitted JSON fields (separate TC messages are collapsed
// into one Atlas point telemetry envelope per observation stream).
type adsbFeed struct {
	cfg FeedConfig
	rng *rng
}

func newADSBFeed(cfg FeedConfig, rng *rng) (*adsbFeed, error) {
	if cfg.IntervalMS <= 0 {
		cfg.IntervalMS = 500
	}
	if cfg.DelayMSMax < cfg.DelayMSMin {
		cfg.DelayMSMin, cfg.DelayMSMax = cfg.DelayMSMax, cfg.DelayMSMin
	}
	if cfg.UncertaintyRadiusM <= 0 {
		cfg.UncertaintyRadiusM = 50
	}
	return &adsbFeed{cfg: cfg, rng: rng}, nil
}

func (f *adsbFeed) run(start time.Time, duration time.Duration, motion Motion) (ObservationSnapshot, error) {
	interval := time.Duration(f.cfg.IntervalMS) * time.Millisecond
	var snap ObservationSnapshot
	snap.ObservationID = f.cfg.ObservationID
	snap.SourceAssetID = f.cfg.SourceAssetID

	for at := time.Duration(0); at <= duration; at += interval {
		if f.rng.chance(f.cfg.DropProbability) {
			snap.SamplesDropped++
			continue
		}
		delay := time.Duration(f.rng.uniform(float64(f.cfg.DelayMSMin), float64(f.cfg.DelayMSMax)+1)) * time.Millisecond
		observedAt := start.Add(at)
		target := motion.StateAt(start, at)
		lat, lon := target.Latitude, target.Longitude
		if f.cfg.PositionNoiseM > 0 {
			heading := f.rng.uniform(0, 360)
			lat, lon = OffsetMeters(lat, lon, heading, f.rng.uniform(0, f.cfg.PositionNoiseM))
		}

		receivedAt := observedAt.Add(delay)
		jsonBytes, err := f.pointTelemetryJSON(observedAt, lat, lon, target.AltitudeM)
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
		return ObservationSnapshot{}, fmt.Errorf("adsb feed %q produced no samples", f.cfg.ObservationID)
	}
	return snap, nil
}

func (f *adsbFeed) pointTelemetryJSON(observedAt time.Time, lat, lon, altM float64) ([]byte, error) {
	data := map[string]any{
		"latitude":  lat,
		"longitude": lon,
	}
	if !containsString(f.cfg.OmitDataFields, "altitude_m") {
		data["altitude_m"] = altM
	}
	if !containsString(f.cfg.OmitDataFields, "uncertainty_radius_m") {
		data["uncertainty_radius_m"] = f.cfg.UncertaintyRadiusM
	}
	payload := map[string]any{
		"latest_telemetry": map[string]any{
			"observed_at": observedAt.UTC().Format(time.RFC3339Nano),
			"kind":        "point",
			"data":        data,
		},
	}
	if len(f.cfg.Identity) > 0 {
		payload["identity"] = f.cfg.Identity
	}
	return json.Marshal(payload)
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
