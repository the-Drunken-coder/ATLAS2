package eval

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/fusion/sim"
)

func checkGroundTruth(result core.Result, expect *GroundTruthExpect) []string {
	if expect == nil {
		return nil
	}
	var failures []string
	withPosition := 0
	bestDistance := math.MaxFloat64
	for _, update := range result.TrackUpdates {
		lat, lon, ok := trackPosition(update.JSON)
		if !ok {
			continue
		}
		withPosition++
		if expect.ToleranceM > 0 {
			dist := sim.HaversineM(lat, lon, expect.Latitude, expect.Longitude)
			if dist < bestDistance {
				bestDistance = dist
			}
		}
	}
	if expect.MinTracksWithPosition > 0 && withPosition < expect.MinTracksWithPosition {
		failures = append(failures, fmt.Sprintf("tracks_with_position: got %d want at least %d", withPosition, expect.MinTracksWithPosition))
	}
	if expect.ToleranceM > 0 && withPosition == 0 {
		failures = append(failures, "no tracks with position to evaluate ground_truth tolerance")
	}
	if expect.ToleranceM > 0 && withPosition > 0 && bestDistance > expect.ToleranceM {
		failures = append(failures, fmt.Sprintf("closest track %.1fm from ground truth exceeds tolerance %.1fm", bestDistance, expect.ToleranceM))
	}
	return failures
}

func trackPosition(trackJSON []byte) (lat, lon float64, ok bool) {
	var root struct {
		Components struct {
			Telemetry struct {
				Latitude  *float64 `json:"latitude"`
				Longitude *float64 `json:"longitude"`
			} `json:"telemetry"`
		} `json:"components"`
	}
	if err := json.Unmarshal(trackJSON, &root); err != nil {
		return 0, 0, false
	}
	if root.Components.Telemetry.Latitude == nil || root.Components.Telemetry.Longitude == nil {
		return 0, 0, false
	}
	return *root.Components.Telemetry.Latitude, *root.Components.Telemetry.Longitude, true
}
