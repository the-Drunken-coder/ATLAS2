package multisensor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
)

var errNoUsablePosition = errors.New("no usable position samples")

const (
	engineName    = "multisensor"
	engineVersion = "v1"
	lobFixWeight  = 0.35
)

// Engine fuses ADS-B points, dual-camera LOB triangulation, and ADS-B identity onto one track.
type Engine struct{}

func (Engine) Name() string    { return engineName }
func (Engine) Version() string { return engineVersion }

func (Engine) Fuse(_ context.Context, batch core.ObservationBatch) (core.Result, error) {
	if len(batch.Observations) == 0 {
		return core.Result{}, nil
	}

	raw := make([][]byte, len(batch.Observations))
	ids := make([]string, len(batch.Observations))
	var inputRefs []core.InputRef
	for i, obs := range batch.Observations {
		raw[i] = obs.JSON
		ids[i] = obs.ObservationID
		inputRefs = append(inputRefs, obs.Ref())
	}

	points, lobs, err := parseObservations(raw, ids)
	if err != nil {
		return core.Result{}, err
	}
	if len(points) == 0 && len(lobs) < 2 {
		return core.Result{}, nil
	}

	fused, err := fusePosition(points, lobs)
	if err != nil {
		if errors.Is(err, errNoUsablePosition) {
			return core.Result{}, nil
		}
		return core.Result{}, err
	}

	trackID := fusedTrackID(batch.Observations)
	provenanceObjectID := "fusion_prov_" + trackID
	observedAt := ""
	if latest := latestObservedFromSamples(points, lobs); !latest.IsZero() {
		observedAt = latest.UTC().Format(time.RFC3339Nano)
	}

	telemetry := map[string]any{
		"latitude":  fused.lat,
		"longitude": fused.lon,
	}
	if fused.altM != nil {
		telemetry["altitude_m"] = *fused.altM
	}
	if fused.uncM > 0 {
		telemetry["uncertainty_radius_m"] = fused.uncM
	}
	if observedAt != "" {
		telemetry["observed_at"] = observedAt
	}

	components := map[string]any{
		"telemetry": telemetry,
		"fusion_summary": map[string]any{
			"source_count":         fused.sourceCount,
			"confidence":           fusionConfidence(fused.sourceCount),
			"provenance_object_id": provenanceObjectID,
			"observed_at":          observedAt,
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
	adsbUsed := 0
	if fused.sourceCount == 1 || fused.sourceCount == 3 {
		adsbUsed = 1
	}
	lobUsed := 0
	if fused.sourceCount == 2 || fused.sourceCount == 3 {
		lobUsed = 2
	}
	provJSON, err := json.Marshal(map[string]any{
		"kind":             "multisensor_adsb_dual_lob",
		"lob_count":        lobUsed,
		"adsb_point_count": adsbUsed,
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

func fusedTrackID(observations []core.ObservationInput) string {
	for _, obs := range observations {
		if len(obs.JSON) > 0 && observationJSONHasPoint(obs.JSON) {
			sum := sha256.Sum256([]byte(obs.ObservationID))
			return "fused_track_" + hex.EncodeToString(sum[:])[:20]
		}
	}
	ids := make([]string, 0, len(observations))
	for _, obs := range observations {
		if obs.ObservationID != "" {
			ids = append(ids, obs.ObservationID)
		}
	}
	sort.Strings(ids)
	seed, err := json.Marshal(ids)
	if err != nil {
		seed = []byte{}
	}
	sum := sha256.Sum256(seed)
	return "fused_track_" + hex.EncodeToString(sum[:])[:20]
}

func observationJSONHasPoint(data []byte) bool {
	var root observationRoot
	if json.Unmarshal(data, &root) != nil {
		return false
	}
	return root.LatestTelemetry.Kind == "point"
}
