package multisensor

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/fusion/eval"
	"github.com/anomalyco/atlas-core/services/fusion/sim"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
)

func scenarioDir(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "scenarios", name)
}

func TestEngineFusesADSBAndDualCameraScenario(t *testing.T) {
	scenario, err := eval.MaterializeSimulation(scenarioDir(t, "moving_adsb_dual_cam"))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	batch, err := scenario.ObservationBatch()
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(batch.Observations) != 3 {
		t.Fatalf("expected 3 observation streams, got %d", len(batch.Observations))
	}

	result, err := (Engine{}).Fuse(context.Background(), batch)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(result.TrackUpdates) != 1 {
		t.Fatalf("expected 1 track, got %d", len(result.TrackUpdates))
	}
	if len(result.Provenance) != 1 || len(result.Provenance[0].Inputs) != 3 {
		t.Fatalf("expected provenance with 3 inputs, got %+v", result.Provenance)
	}

	var track struct {
		Components struct {
			Telemetry struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"telemetry"`
		} `json:"components"`
		Extra struct {
			Identity map[string]any `json:"identity"`
		} `json:"extra"`
	}
	if err := json.Unmarshal(result.TrackUpdates[0].JSON, &track); err != nil {
		t.Fatalf("unmarshal track: %v", err)
	}
	if track.Extra.Identity["icao24"] != "a1b2c3" {
		t.Fatalf("expected ADS-B identity on track, got %+v", track.Extra.Identity)
	}
	if scenario.GroundTruth != nil {
		errM := sim.HaversineM(
			track.Components.Telemetry.Latitude, track.Components.Telemetry.Longitude,
			scenario.GroundTruth.Latitude, scenario.GroundTruth.Longitude,
		)
		if errM > scenario.GroundTruth.ToleranceM {
			t.Fatalf("fused position %.1fm from ground truth (tol %.1fm)", errM, scenario.GroundTruth.ToleranceM)
		}
	}

	validator, err := protocolvalidation.New()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	if issues := validator.ValidateEntity(&model.Entity{
		EntityID: result.TrackUpdates[0].TrackID,
		Type:     model.EntityTypeTrack,
		JSON:     result.TrackUpdates[0].JSON,
	}); len(issues) > 0 {
		t.Fatalf("protocol issues: %+v", issues)
	}
}

func TestEngineProtocolValidOnSyntheticBatch(t *testing.T) {
	latest := time.Date(2026, 1, 1, 12, 1, 0, 0, time.UTC)
	batch := core.NewObservationBatch([]core.ObservationInput{
		{
			ObservationID: "obs_adsb", SourceAssetID: "asset_adsb",
			LatestTelemetryAt: latest, UpdatedAt: latest, Version: 1,
			JSON: json.RawMessage(`{"identity":{"kind":"aircraft","icao24":"a1b2c3","callsign":"TEST123"},"latest_telemetry":{"observed_at":"2026-01-01T12:01:00Z","kind":"point","data":{"latitude":40.7026,"longitude":-73.9610,"altitude_m":1200,"uncertainty_radius_m":75}}}`),
		},
		{
			ObservationID: "obs_cam1", SourceAssetID: "asset_cam1",
			LatestTelemetryAt: latest, UpdatedAt: latest, Version: 1,
			JSON: json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T12:01:00Z","kind":"line_of_bearing","data":{"observer_latitude":40.71,"observer_longitude":-74.01,"observer_altitude_m":28,"azimuth_deg":101.3,"elevation_deg":15.6}}}`),
		},
		{
			ObservationID: "obs_cam2", SourceAssetID: "asset_cam2",
			LatestTelemetryAt: latest, UpdatedAt: latest, Version: 1,
			JSON: json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T12:01:00Z","kind":"line_of_bearing","data":{"observer_latitude":40.69,"observer_longitude":-74.02,"observer_altitude_m":32,"azimuth_deg":118.5,"elevation_deg":14.2}}}`),
		},
	}, core.Checkpoint{})
	result, err := (Engine{}).Fuse(context.Background(), batch)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(result.TrackUpdates) != 1 {
		t.Fatalf("expected one track")
	}
}
