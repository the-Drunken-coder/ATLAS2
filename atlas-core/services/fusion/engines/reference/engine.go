package reference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
)

const engineName = "reference"
const engineVersion = "v1"

// Engine projects each point observation into a dedicated track (integration reference model).
type Engine struct{}

func (Engine) Name() string {
	return engineName
}

func (Engine) Version() string {
	return engineVersion
}

func (Engine) Fuse(_ context.Context, batch core.ObservationBatch) (core.Result, error) {
	var result core.Result
	for _, obs := range batch.Observations {
		point, ok, err := pointTelemetry(obs.JSON)
		if err != nil {
			return core.Result{}, err
		}
		if !ok {
			continue
		}
		trackID := trackID(obs.ObservationID)
		provenanceObjectID := "fusion_prov_" + trackID
		trackTelemetry := trackTelemetry{
			Latitude:  point.Latitude,
			Longitude: point.Longitude,
		}
		if !obs.LatestTelemetryAt.IsZero() {
			trackTelemetry.ObservedAt = obs.LatestTelemetryAt.UTC().Format(time.RFC3339Nano)
		}
		trackTelemetry.AltitudeM = point.AltitudeM
		trackTelemetry.UncertaintyRadiusM = point.UncertaintyRadiusM
		fusionSummary := trackFusionSummary{
			SourceCount:        1,
			Confidence:         1,
			ProvenanceObjectID: provenanceObjectID,
			ObservedAt:         trackTelemetry.ObservedAt,
		}
		trackJSON, err := json.Marshal(trackJSONPayload{
			Components: trackComponents{
				Telemetry:     trackTelemetry,
				FusionSummary: fusionSummary,
			},
			CustomReferenceFusion: fusionMeta{
				ObservationID: obs.ObservationID,
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
		provenanceJSON, err := json.Marshal(provenancePayload{
			Kind: "reference_point_projection",
		})
		if err != nil {
			return core.Result{}, err
		}
		result.Provenance = append(result.Provenance, core.ProvenanceRecord{
			TrackID:       trackID,
			EngineName:    engineName,
			EngineVersion: engineVersion,
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

func trackID(observationID string) string {
	sum := sha256.Sum256([]byte(observationID))
	return "ref_track_" + hex.EncodeToString(sum[:])[:24]
}
