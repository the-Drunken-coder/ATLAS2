package multisensor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/fusion/sim"
)

const (
	engineName    = "multisensor"
	engineVersion = "v1"
	lobFixWeight  = 0.35
)

// Engine fuses ADS-B points, dual-camera LOB triangulation, and ADS-B identity onto one track.
type Engine struct{}

func (Engine) Name() string  { return engineName }
func (Engine) Version() string { return engineVersion }

func (Engine) Fuse(_ context.Context, batch core.ObservationBatch) (core.Result, error) {
	if len(batch.Observations) == 0 {
		return core.Result{}, nil
	}

	raw := make([][]byte, len(batch.Observations))
	ids := make([]string, len(batch.Observations))
	var latestObserved time.Time
	var inputRefs []core.InputRef
	for i, obs := range batch.Observations {
		raw[i] = obs.JSON
		ids[i] = obs.ObservationID
		inputRefs = append(inputRefs, obs.Ref())
		if !obs.LatestTelemetryAt.IsZero() && obs.LatestTelemetryAt.After(latestObserved) {
			latestObserved = obs.LatestTelemetryAt
		}
	}

	points, lobs, err := parseObservations(raw, ids)
	if err != nil {
		return core.Result{}, err
	}
	if len(points) == 0 && len(lobs) < 2 {
		return core.Result{}, nil
	}

	lat, lon, altM, uncM, err := fusePosition(points, lobs)
	if err != nil {
		return core.Result{}, err
	}

	trackID := fusedTrackID(batch.Observations)
	provenanceObjectID := "fusion_prov_" + trackID
	observedAt := ""
	if !latestObserved.IsZero() {
		observedAt = latestObserved.UTC().Format(time.RFC3339Nano)
	}

	telemetry := map[string]any{
		"latitude":  lat,
		"longitude": lon,
	}
	if altM != nil {
		telemetry["altitude_m"] = *altM
	}
	if uncM > 0 {
		telemetry["uncertainty_radius_m"] = uncM
	}
	if observedAt != "" {
		telemetry["observed_at"] = observedAt
	}

	components := map[string]any{
		"telemetry": telemetry,
		"fusion_summary": map[string]any{
			"source_count":         len(points) + len(lobs),
			"confidence":           fusionConfidence(points, lobs),
			"provenance_object_id": provenanceObjectID,
			"observed_at":            observedAt,
		},
	}
	payload := map[string]any{"components": components}
	if identity := richestIdentity(points); len(identity) > 0 {
		payload["extra"] = map[string]any{"identity": identity}
	}

	trackJSON, err := json.Marshal(payload)
	if err != nil {
		return core.Result{}, err
	}
	provJSON, err := json.Marshal(map[string]any{
		"kind":              "multisensor_adsb_dual_lob",
		"lob_count":         len(lobs),
		"adsb_point_count":  len(points),
	})
	if err != nil {
		return core.Result{}, err
	}

	return core.Result{
		TrackUpdates: []core.TrackUpdate{{
			TrackID:            trackID,
			JSON:               trackJSON,
			ProvenanceObjectID: provenanceObjectID,
		}},
		Provenance: []core.ProvenanceRecord{{
			TrackID:       trackID,
			EngineName:    engineName,
			EngineVersion: engineVersion,
			Inputs:        inputRefs,
			JSON:          provJSON,
		}},
	}, nil
}

func fusePosition(points []pointSample, lobs []lobSample) (lat, lon float64, altM *float64, uncM float64, err error) {
	var adsbLat, adsbLon float64
	var adsbAlt *float64
	var adsbUnc float64
	var hasADSB bool
	if len(points) > 0 {
		// Latest ADS-B point in batch (observations are time-ordered).
		last := points[len(points)-1]
		adsbLat, adsbLon = last.lat, last.lon
		adsbAlt = last.altM
		adsbUnc = last.uncertaintyM
		hasADSB = true
	}

	var lobLat, lobLon float64
	var lobAlt float64
	var hasLOB bool
	if len(lobs) >= 2 {
		a, b := lobs[0], lobs[1]
		latT, lonT, altT, ok := sim.TriangulateLOB(
			a.observerLat, a.observerLon, a.observerAltM, a.azimuthDeg, a.elevationDeg,
			b.observerLat, b.observerLon, b.observerAltM, b.azimuthDeg, b.elevationDeg,
		)
		if ok {
			lobLat, lobLon, lobAlt = latT, lonT, altT
			hasLOB = true
		}
	}

	switch {
	case hasADSB && hasLOB:
		lat = adsbLat*(1-lobFixWeight) + lobLat*lobFixWeight
		lon = adsbLon*(1-lobFixWeight) + lobLon*lobFixWeight
		altM = adsbAlt
		if altM == nil && lobAlt != 0 {
			v := lobAlt
			altM = &v
		}
		uncM = adsbUnc
		lobErr := sim.HaversineM(lobLat, lobLon, adsbLat, adsbLon)
		if lobErr > uncM {
			uncM = lobErr
		}
	case hasADSB:
		lat, lon, altM, uncM = adsbLat, adsbLon, adsbAlt, adsbUnc
	case hasLOB:
		lat, lon = lobLat, lobLon
		v := lobAlt
		altM = &v
		uncM = 200
	default:
		return 0, 0, nil, 0, fmt.Errorf("no usable position samples")
	}
	return lat, lon, altM, uncM, nil
}

func fusionConfidence(points []pointSample, lobs []lobSample) float64 {
	sources := len(points) + len(lobs)
	if sources >= 3 {
		return 0.92
	}
	if sources == 2 {
		return 0.75
	}
	return 0.5
}

func fusedTrackID(observations []core.ObservationInput) string {
	for _, obs := range observations {
		if len(obs.JSON) > 0 && observationJSONHasPoint(obs.JSON) {
			sum := sha256.Sum256([]byte(obs.ObservationID))
			return "fused_track_" + hex.EncodeToString(sum[:])[:20]
		}
	}
	sum := sha256.Sum256([]byte("multisensor"))
	return "fused_track_" + hex.EncodeToString(sum[:])[:20]
}

func observationJSONHasPoint(data []byte) bool {
	var root observationRoot
	if json.Unmarshal(data, &root) != nil {
		return false
	}
	return root.LatestTelemetry.Kind == "point"
}
