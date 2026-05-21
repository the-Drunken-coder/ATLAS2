package engines

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
)

type ReferenceEngine struct{}

func (ReferenceEngine) Name() string {
	return "reference"
}

func (ReferenceEngine) Version() string {
	return "v1"
}

func (ReferenceEngine) Fuse(_ context.Context, batch core.ObservationBatch) (core.Result, error) {
	var result core.Result
	for _, obs := range batch.Observations {
		point, ok, err := pointTelemetry(obs.JSON)
		if err != nil {
			return core.Result{}, err
		}
		if !ok {
			continue
		}
		trackID := referenceTrackID(obs.ObservationID)
		provenanceObjectID := "fusion_prov_" + trackID
		var telemetryAt string
		var telemetry map[string]any
		if obs.LatestTelemetryAt != nil {
			telemetryAt = obs.LatestTelemetryAt.UTC().Format(time.RFC3339Nano)
			telemetry = map[string]any{
				"observed_at": telemetryAt,
				"latitude":    point.Latitude,
				"longitude":   point.Longitude,
			}
		} else {
			telemetry = map[string]any{
				"latitude":  point.Latitude,
				"longitude": point.Longitude,
			}
		}
		if point.AltitudeM != nil {
			telemetry["altitude_m"] = *point.AltitudeM
		}
		if point.UncertaintyRadiusM != nil {
			telemetry["uncertainty_radius_m"] = *point.UncertaintyRadiusM
		}
		fusionSummary := map[string]any{
			"source_count":         1,
			"confidence":           1,
			"provenance_object_id": provenanceObjectID,
		}
		if telemetryAt != "" {
			fusionSummary["observed_at"] = telemetryAt
		}
		trackJSON, err := json.Marshal(map[string]any{
			"components": map[string]any{
				"telemetry":      telemetry,
				"fusion_summary": fusionSummary,
			},
			"custom_reference_fusion": map[string]any{
				"observation_id": obs.ObservationID,
			},
		})
		if err != nil {
			return core.Result{}, err
		}
		result.TrackUpdates = append(result.TrackUpdates, core.TrackUpdate{
			TrackID:            trackID,
			JSON:               trackJSON,
			ProvenanceObjectID: provenanceObjectID,
		})
		provenanceJSON, err := json.Marshal(map[string]any{
			"kind": "reference_point_projection",
		})
		if err != nil {
			return core.Result{}, err
		}
		result.Provenance = append(result.Provenance, core.ProvenanceRecord{
			TrackID:       trackID,
			EngineName:    "reference",
			EngineVersion: "v1",
			Inputs:        []core.InputRef{obs.Ref()},
			JSON:          provenanceJSON,
		})
	}
	return result, nil
}

type telemetryRoot struct {
	LatestTelemetry struct {
		Kind string `json:"kind"`
		Data struct {
			Latitude           *float64 `json:"latitude"`
			Longitude          *float64 `json:"longitude"`
			AltitudeM          *float64 `json:"altitude_m,omitempty"`
			UncertaintyRadiusM *float64 `json:"uncertainty_radius_m,omitempty"`
		} `json:"data"`
	} `json:"latest_telemetry"`
}

type pointData struct {
	Latitude           float64
	Longitude          float64
	AltitudeM          *float64
	UncertaintyRadiusM *float64
}

func pointTelemetry(data []byte) (pointData, bool, error) {
	var root telemetryRoot
	if err := json.Unmarshal(data, &root); err != nil {
		return pointData{}, false, fmt.Errorf("parse observation json: %w", err)
	}
	if root.LatestTelemetry.Kind != "point" {
		return pointData{}, false, nil
	}
	if root.LatestTelemetry.Data.Latitude == nil || root.LatestTelemetry.Data.Longitude == nil {
		return pointData{}, false, nil
	}
	return pointData{
		Latitude:           *root.LatestTelemetry.Data.Latitude,
		Longitude:          *root.LatestTelemetry.Data.Longitude,
		AltitudeM:          root.LatestTelemetry.Data.AltitudeM,
		UncertaintyRadiusM: root.LatestTelemetry.Data.UncertaintyRadiusM,
	}, true, nil
}

func referenceTrackID(observationID string) string {
	sum := sha256.Sum256([]byte(observationID))
	return "ref_track_" + hex.EncodeToString(sum[:])[:24]
}
