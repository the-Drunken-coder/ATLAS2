package multisensor

import (
	"encoding/json"
	"fmt"
	"time"
)

type pointSample struct {
	observationID string
	lat           float64
	lon           float64
	altM          *float64
	uncertaintyM  float64
	identity      map[string]any
	observedAt    time.Time
}

type lobSample struct {
	observationID string
	observerLat   float64
	observerLon   float64
	observerAltM  float64
	azimuthDeg    float64
	elevationDeg  float64
	observedAt    time.Time
}

type observationRoot struct {
	Identity        map[string]any `json:"identity"`
	LatestTelemetry struct {
		Kind       string `json:"kind"`
		ObservedAt string `json:"observed_at"`
		Data       struct {
			Latitude           *float64 `json:"latitude"`
			Longitude          *float64 `json:"longitude"`
			AltitudeM          *float64 `json:"altitude_m"`
			UncertaintyRadiusM *float64 `json:"uncertainty_radius_m"`
			ObserverLatitude   *float64 `json:"observer_latitude"`
			ObserverLongitude  *float64 `json:"observer_longitude"`
			ObserverAltitudeM  *float64 `json:"observer_altitude_m"`
			AzimuthDeg         *float64 `json:"azimuth_deg"`
			ElevationDeg       *float64 `json:"elevation_deg"`
		} `json:"data"`
	} `json:"latest_telemetry"`
}

func parseObservations(batchJSON [][]byte, observationIDs []string) ([]pointSample, []lobSample, error) {
	var points []pointSample
	var lobs []lobSample
	for i, raw := range batchJSON {
		var root observationRoot
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, nil, fmt.Errorf("parse observation json: %w", err)
		}
		observedAt, err := parseTelemetryObservedAt(root.LatestTelemetry.ObservedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("observations[%d].latest_telemetry.observed_at: %w", i, err)
		}
		obsID := observationIDs[i]
		switch root.LatestTelemetry.Kind {
		case "point":
			if root.LatestTelemetry.Data.Latitude == nil || root.LatestTelemetry.Data.Longitude == nil {
				continue
			}
			unc := 75.0
			if root.LatestTelemetry.Data.UncertaintyRadiusM != nil {
				unc = *root.LatestTelemetry.Data.UncertaintyRadiusM
			}
			points = append(points, pointSample{
				observationID: obsID,
				lat:           *root.LatestTelemetry.Data.Latitude,
				lon:           *root.LatestTelemetry.Data.Longitude,
				altM:          root.LatestTelemetry.Data.AltitudeM,
				uncertaintyM:  unc,
				identity:      root.Identity,
				observedAt:    observedAt,
			})
		case "line_of_bearing":
			d := root.LatestTelemetry.Data
			if d.ObserverLatitude == nil || d.ObserverLongitude == nil || d.AzimuthDeg == nil || d.ElevationDeg == nil {
				continue
			}
			alt := 0.0
			if d.ObserverAltitudeM != nil {
				alt = *d.ObserverAltitudeM
			}
			lobs = append(lobs, lobSample{
				observationID: obsID,
				observerLat:   *d.ObserverLatitude,
				observerLon:   *d.ObserverLongitude,
				observerAltM:  alt,
				azimuthDeg:    *d.AzimuthDeg,
				elevationDeg:  *d.ElevationDeg,
				observedAt:    observedAt,
			})
		}
	}
	return points, lobs, nil
}

func parseTelemetryObservedAt(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t, err = time.Parse(time.RFC3339, raw)
	}
	return t, err
}

func latestObservedFromSamples(points []pointSample, lobs []lobSample) time.Time {
	var latest time.Time
	for _, p := range points {
		if !p.observedAt.IsZero() && p.observedAt.After(latest) {
			latest = p.observedAt
		}
	}
	for _, l := range lobs {
		if !l.observedAt.IsZero() && l.observedAt.After(latest) {
			latest = l.observedAt
		}
	}
	return latest
}

func newestPointSample(points []pointSample) (pointSample, bool) {
	if len(points) == 0 {
		return pointSample{}, false
	}
	best := points[0]
	for _, p := range points[1:] {
		if p.observedAt.After(best.observedAt) {
			best = p
			continue
		}
		if p.observedAt.Equal(best.observedAt) {
			best = p
		}
	}
	return best, true
}

func richestIdentity(points []pointSample) map[string]any {
	var best map[string]any
	bestScore := -1
	for _, p := range points {
		score := identityScore(p.identity)
		if score > bestScore {
			bestScore = score
			best = p.identity
		}
	}
	return best
}

func identityScore(identity map[string]any) int {
	if len(identity) == 0 {
		return 0
	}
	score := len(identity)
	if _, ok := identity["icao24"]; ok {
		score += 10
	}
	if _, ok := identity["callsign"]; ok {
		score += 8
	}
	if kind, ok := identity["kind"].(string); ok && kind != "" {
		score += 2
	}
	return score
}
