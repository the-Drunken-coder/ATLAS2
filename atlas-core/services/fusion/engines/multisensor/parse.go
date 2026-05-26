package multisensor

import (
	"encoding/json"
	"fmt"
)

type pointSample struct {
	observationID string
	lat           float64
	lon           float64
	altM          *float64
	uncertaintyM  float64
	identity      map[string]any
}

type lobSample struct {
	observationID string
	observerLat   float64
	observerLon   float64
	observerAltM  float64
	azimuthDeg    float64
	elevationDeg  float64
}

type observationRoot struct {
	Identity        map[string]any `json:"identity"`
	LatestTelemetry struct {
		Kind string `json:"kind"`
		Data struct {
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
			})
		}
	}
	return points, lobs, nil
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
