package sim

import (
	"encoding/json"
	"testing"
)

func TestRunDualFeedIsDeterministicAndMovesTarget(t *testing.T) {
	def := Definition{
		Name:         "dual",
		Seed:         7,
		StartRFC3339: "2026-01-01T12:00:00Z",
		DurationSec:  30,
		Target: Motion{
			InitialLat:  40.0,
			InitialLon:  -74.0,
			InitialAltM: 1000,
			SpeedMPS:    40,
			HeadingDeg:  90,
		},
		Feeds: []FeedConfig{
			{
				Type:          "adsb",
				ObservationID: "obs_adsb",
				SourceAssetID: "asset_adsb",
				IntervalMS:    500,
				DelayMSMin:    50,
				DelayMSMax:    100,
			},
			{
				Type:          "camera_lob",
				ObservationID: "obs_cam",
				SourceAssetID: "asset_cam",
				IntervalMS:    1000,
				ObserverLat:   40.01,
				ObserverLon:   -74.01,
				ObserverAltM:  20,
			},
		},
	}
	first, err := Run(def)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	second, err := Run(def)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(first.Observations) != 2 || len(second.Observations) != 2 {
		t.Fatalf("expected two feeds, got %d and %d", len(first.Observations), len(second.Observations))
	}
	if string(first.Observations[0].JSON) != string(second.Observations[0].JSON) {
		t.Fatal("expected deterministic ADS-B JSON")
	}
	moved := HaversineM(def.Target.InitialLat, def.Target.InitialLon, first.GroundTruth.Latitude, first.GroundTruth.Longitude)
	if moved < 100 {
		t.Fatalf("expected target to move materially, got %.1fm", moved)
	}

	var adsbPayload struct {
		LatestTelemetry struct {
			Kind string `json:"kind"`
		} `json:"latest_telemetry"`
	}
	if err := json.Unmarshal(first.Observations[0].JSON, &adsbPayload); err != nil {
		t.Fatalf("unmarshal adsb json: %v", err)
	}
	if adsbPayload.LatestTelemetry.Kind != "point" {
		t.Fatalf("expected point telemetry, got %q", adsbPayload.LatestTelemetry.Kind)
	}
	var lobPayload struct {
		LatestTelemetry struct {
			Kind string `json:"kind"`
		} `json:"latest_telemetry"`
	}
	if err := json.Unmarshal(first.Observations[1].JSON, &lobPayload); err != nil {
		t.Fatalf("unmarshal lob json: %v", err)
	}
	if lobPayload.LatestTelemetry.Kind != "line_of_bearing" {
		t.Fatalf("expected lob telemetry, got %q", lobPayload.LatestTelemetry.Kind)
	}
}
