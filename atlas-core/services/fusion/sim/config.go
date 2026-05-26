package sim

import "time"

// Definition describes a target-centric scenario: one moving target and sensor feeds.
type Definition struct {
	Name         string       `json:"name"`
	Seed         int64        `json:"seed"`
	Start        time.Time    `json:"-"`
	StartRFC3339 string       `json:"start"`
	DurationSec  float64      `json:"duration_sec"`
	Target       Motion       `json:"target"`
	Feeds        []FeedConfig `json:"feeds"`
}

// FeedConfig is a sensor feed simulated from the same target motion.
type FeedConfig struct {
	Type string `json:"type"`

	ObservationID string `json:"observation_id"`
	SourceAssetID string `json:"source_asset_id"`

	IntervalMS int `json:"interval_ms"`

	// ADS-B style feed (1090ES position reports ~2 Hz with delivery delay and drops).
	DelayMSMin         int      `json:"delay_ms_min,omitempty"`
	DelayMSMax         int      `json:"delay_ms_max,omitempty"`
	DropProbability    float64  `json:"drop_probability,omitempty"`
	PositionNoiseM     float64  `json:"position_noise_m,omitempty"`
	OmitDataFields     []string `json:"omit_data_fields,omitempty"`
	UncertaintyRadiusM float64  `json:"uncertainty_radius_m,omitempty"`
	// Identity is attached to observation JSON (typical for ADS-B: icao24, callsign, etc.).
	Identity map[string]any `json:"identity,omitempty"`

	// Camera line-of-bearing feed.
	ObserverLat       float64 `json:"observer_latitude,omitempty"`
	ObserverLon       float64 `json:"observer_longitude,omitempty"`
	ObserverAltM      float64 `json:"observer_altitude_m,omitempty"`
	BearingNoiseDeg   float64 `json:"bearing_noise_deg,omitempty"`
	ElevationNoiseDeg float64 `json:"elevation_noise_deg,omitempty"`
}

// ObservationSnapshot is the latest row state for one observation stream after simulation.
type ObservationSnapshot struct {
	ObservationID     string
	SourceAssetID     string
	LatestTelemetryAt time.Time
	UpdatedAt         time.Time
	Version           int
	JSON              []byte
	SamplesEmitted    int
	SamplesDropped    int
}

// Result is produced by running a Definition.
type Result struct {
	Name         string
	Observations []ObservationSnapshot
	GroundTruth  TargetState
}
