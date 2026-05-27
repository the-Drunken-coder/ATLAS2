package multisensor

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/fusion/sim"
)

func TestAltitudeFromADSBAndLOBIncludesZeroMeters(t *testing.T) {
	zero := 0.0
	altM := altitudeFromADSBAndLOB(nil, &zero)
	if altM == nil {
		t.Fatal("expected altitude when LOB reports 0m and ADS-B altitude is missing")
	}
	if *altM != 0 {
		t.Fatalf("expected 0m altitude, got %v", *altM)
	}
}

func TestAltitudeFromADSBAndLOBPrefersADSB(t *testing.T) {
	adsb := 1200.0
	lob := 0.0
	altM := altitudeFromADSBAndLOB(&adsb, &lob)
	if altM == nil || *altM != 1200 {
		t.Fatalf("expected ADS-B altitude, got %v", altM)
	}
}

func TestEngineIncludesAltitudeFromLOBWhenADSBPointOmitsAltitude(t *testing.T) {
	latest := time.Date(2026, 1, 1, 12, 1, 0, 0, time.UTC)
	batch := core.NewObservationBatch([]core.ObservationInput{
		{
			ObservationID: "obs_adsb", SourceAssetID: "asset_adsb",
			LatestTelemetryAt: latest, UpdatedAt: latest, Version: 1,
			JSON: json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T12:01:00Z","kind":"point","data":{"latitude":40.0,"longitude":-74.0,"uncertainty_radius_m":50}}}`),
		},
		{
			ObservationID: "obs_cam1", SourceAssetID: "asset_cam1",
			LatestTelemetryAt: latest, UpdatedAt: latest, Version: 1,
			JSON: json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T12:01:00Z","kind":"line_of_bearing","data":{"observer_latitude":40.0,"observer_longitude":-74.0,"observer_altitude_m":0,"azimuth_deg":90,"elevation_deg":0}}}`),
		},
		{
			ObservationID: "obs_cam2", SourceAssetID: "asset_cam2",
			LatestTelemetryAt: latest, UpdatedAt: latest, Version: 1,
			JSON: json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T12:01:00Z","kind":"line_of_bearing","data":{"observer_latitude":40.0,"observer_longitude":-73.99,"observer_altitude_m":0,"azimuth_deg":270,"elevation_deg":0}}}`),
		},
	}, core.Checkpoint{})

	result, err := (Engine{}).Fuse(context.Background(), batch)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(result.TrackUpdates) != 1 {
		t.Fatalf("expected one track, got %d", len(result.TrackUpdates))
	}
	var track struct {
		Components struct {
			Telemetry struct {
				AltitudeM *float64 `json:"altitude_m"`
			} `json:"telemetry"`
		} `json:"components"`
	}
	if err := json.Unmarshal(result.TrackUpdates[0].JSON, &track); err != nil {
		t.Fatalf("unmarshal track: %v", err)
	}
	if track.Components.Telemetry.AltitudeM == nil {
		t.Fatal("expected altitude_m on fused track when LOB supplies altitude and ADS-B omits it")
	}
	if math.Abs(*track.Components.Telemetry.AltitudeM) > 5 {
		t.Fatalf("expected near sea-level altitude from LOB, got %v", *track.Components.Telemetry.AltitudeM)
	}
}

func TestFusePositionReturnsNoUsablePositionWhenSamplesMissing(t *testing.T) {
	_, err := fusePosition(nil, nil)
	if !errors.Is(err, errNoUsablePosition) {
		t.Fatalf("expected errNoUsablePosition, got %v", err)
	}
}

func TestEngineObservedAtFromParsedSamplesOnly(t *testing.T) {
	older := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 1, 12, 5, 0, 0, time.UTC)
	batch := core.NewObservationBatch([]core.ObservationInput{
		{
			ObservationID: "obs_point", SourceAssetID: "asset_adsb",
			LatestTelemetryAt: older, UpdatedAt: older, Version: 1,
			JSON: json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T12:00:00Z","kind":"point","data":{"latitude":40.0,"longitude":-74.0}}}`),
		},
		{
			ObservationID: "obs_ignored", SourceAssetID: "asset_other",
			LatestTelemetryAt: newer, UpdatedAt: newer, Version: 1,
			JSON: json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T12:05:00Z","kind":"point","data":{"latitude":41.0}}}`),
		},
	}, core.Checkpoint{})

	result, err := (Engine{}).Fuse(context.Background(), batch)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(result.TrackUpdates) != 1 {
		t.Fatalf("expected one track, got %d", len(result.TrackUpdates))
	}
	var track struct {
		Components struct {
			Telemetry struct {
				ObservedAt string `json:"observed_at"`
			} `json:"telemetry"`
			FusionSummary struct {
				ObservedAt string `json:"observed_at"`
			} `json:"fusion_summary"`
		} `json:"components"`
	}
	if err := json.Unmarshal(result.TrackUpdates[0].JSON, &track); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := "2026-01-01T12:00:00Z"
	if track.Components.Telemetry.ObservedAt != want {
		t.Fatalf("telemetry observed_at: got %q want %q", track.Components.Telemetry.ObservedAt, want)
	}
	if track.Components.FusionSummary.ObservedAt != want {
		t.Fatalf("fusion_summary observed_at: got %q want %q", track.Components.FusionSummary.ObservedAt, want)
	}
}

func TestFusePositionUsesNewestPointByObservedAt(t *testing.T) {
	older := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 1, 12, 10, 0, 0, time.UTC)
	points := []pointSample{
		{lat: 40.0, lon: -74.0, uncertaintyM: 50, observedAt: newer},
		{lat: 41.0, lon: -73.0, uncertaintyM: 50, observedAt: older},
	}
	fused, err := fusePosition(points, nil)
	if err != nil {
		t.Fatalf("fusePosition: %v", err)
	}
	if fused.lat != 40.0 || fused.lon != -74.0 {
		t.Fatalf("expected newest point position (40,-74), got (%v,%v)", fused.lat, fused.lon)
	}
}

func TestFusePositionBlendsLongitudeAcrossAntimeridian(t *testing.T) {
	points := []pointSample{{lat: 0, lon: 179, uncertaintyM: 50}}
	lobs := []lobSample{
		{observerLat: 0, observerLon: 170, observerAltM: 0, azimuthDeg: 90, elevationDeg: 0},
		{observerLat: 0, observerLon: -170, observerAltM: 0, azimuthDeg: 270, elevationDeg: 0},
	}
	fused, err := fusePosition(points, lobs)
	if err != nil {
		t.Fatalf("fusePosition: %v", err)
	}
	linearLon := 179*(1-lobFixWeight) + (-179)*lobFixWeight
	if math.Abs(fused.lon-linearLon) < 1 {
		t.Fatalf("expected antimeridian-safe blend, got lon=%.4f (linear would be %.4f)", fused.lon, linearLon)
	}
	if fused.adsbUsed != 1 || fused.lobsUsed != 2 {
		t.Fatalf("expected 1 ADS-B + 2 LOB used, got adsb=%d lobs=%d", fused.adsbUsed, fused.lobsUsed)
	}
}

func contributingLOBCount(lobs []lobSample) (int, bool) {
	usedIdx := make(map[int]struct{})
	for i := 0; i < len(lobs); i++ {
		for j := i + 1; j < len(lobs); j++ {
			a, b := lobs[i], lobs[j]
			_, _, _, _, triOK := sim.TriangulateLOB(
				a.observerLat, a.observerLon, a.observerAltM, a.azimuthDeg, a.elevationDeg,
				b.observerLat, b.observerLon, b.observerAltM, b.azimuthDeg, b.elevationDeg,
			)
			if !triOK {
				continue
			}
			usedIdx[i] = struct{}{}
			usedIdx[j] = struct{}{}
		}
	}
	if len(usedIdx) < 2 {
		return 0, false
	}
	return len(usedIdx), true
}

func TestTriangulateLOBsCountsOnlyContributingLOBs(t *testing.T) {
	cases := []struct {
		name string
		lobs []lobSample
	}{
		{
			name: "two_lob_pair",
			lobs: []lobSample{
				{observerLat: 40.0, observerLon: -74.0, observerAltM: 0, azimuthDeg: 90, elevationDeg: 0},
				{observerLat: 40.0, observerLon: -73.99, observerAltM: 0, azimuthDeg: 270, elevationDeg: 0},
			},
		},
		{
			name: "collinear_first_pair_skipped",
			lobs: []lobSample{
				{observerLat: 40.0, observerLon: -74.0, observerAltM: 0, azimuthDeg: 90, elevationDeg: 0},
				{observerLat: 40.0, observerLon: -74.0, observerAltM: 0, azimuthDeg: 90, elevationDeg: 0},
				{observerLat: 40.0, observerLon: -73.99, observerAltM: 0, azimuthDeg: 270, elevationDeg: 0},
			},
		},
		{
			name: "four_lob_mixed_geometry",
			lobs: []lobSample{
				{observerLat: 40.0, observerLon: -74.0, observerAltM: 0, azimuthDeg: 90, elevationDeg: 0},
				{observerLat: 40.0, observerLon: -74.0, observerAltM: 0, azimuthDeg: 90, elevationDeg: 0},
				{observerLat: 40.0, observerLon: -73.99, observerAltM: 0, azimuthDeg: 270, elevationDeg: 0},
				{observerLat: 40.0, observerLon: -74.01, observerAltM: 0, azimuthDeg: 90, elevationDeg: 0},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantUsed, wantOK := contributingLOBCount(tc.lobs)
			_, _, _, gotUsed, gotOK := triangulateLOBs(tc.lobs)
			if gotOK != wantOK {
				t.Fatalf("ok: got %v want %v", gotOK, wantOK)
			}
			if !wantOK {
				return
			}
			if gotUsed != wantUsed {
				t.Fatalf("lobsUsed: got %d want %d (len(lobs)=%d)", gotUsed, wantUsed, len(tc.lobs))
			}
			// Guard against the old len(lobs) fallback when some LOBs never triangulate.
			if wantUsed < len(tc.lobs) && gotUsed == len(tc.lobs) {
				t.Fatalf("regression: counted all %d LOBs but only %d contributed", len(tc.lobs), wantUsed)
			}
		})
	}
}

func TestTriangulateLOBsUsesLaterPairsWhenFirstIsCollinear(t *testing.T) {
	lobs := []lobSample{
		{observerLat: 40.0, observerLon: -74.0, observerAltM: 0, azimuthDeg: 90, elevationDeg: 0},
		{observerLat: 40.0, observerLon: -74.0, observerAltM: 0, azimuthDeg: 90, elevationDeg: 0},
		{observerLat: 40.0, observerLon: -73.99, observerAltM: 0, azimuthDeg: 270, elevationDeg: 0},
	}
	_, _, _, used, ok := triangulateLOBs(lobs)
	if !ok {
		t.Fatal("expected triangulation when collinear pair is skipped")
	}
	if used != 3 {
		t.Fatalf("expected all 3 LOBs counted, got %d", used)
	}
}

func TestEngineReturnsEmptyResultForSingleLOB(t *testing.T) {
	latest := time.Date(2026, 1, 1, 12, 1, 0, 0, time.UTC)
	batch := core.NewObservationBatch([]core.ObservationInput{
		{
			ObservationID: "obs_cam1", SourceAssetID: "asset_cam1",
			LatestTelemetryAt: latest, UpdatedAt: latest, Version: 1,
			JSON: json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T12:01:00Z","kind":"line_of_bearing","data":{"observer_latitude":40.0,"observer_longitude":-74.0,"observer_altitude_m":0,"azimuth_deg":90,"elevation_deg":0}}}`),
		},
	}, core.Checkpoint{})

	result, err := (Engine{}).Fuse(context.Background(), batch)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(result.TrackUpdates) != 0 {
		t.Fatalf("expected no tracks for single LOB, got %d", len(result.TrackUpdates))
	}
}
