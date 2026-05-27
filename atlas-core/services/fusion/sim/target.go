package sim

import "time"

// TargetState is ground truth for one target at an instant.
type TargetState struct {
	Time      time.Time
	Latitude  float64
	Longitude float64
	AltitudeM float64
}

// Motion describes constant-velocity movement over a flat local earth model.
type Motion struct {
	InitialLat  float64 `json:"initial_latitude"`
	InitialLon  float64 `json:"initial_longitude"`
	InitialAltM float64 `json:"initial_altitude_m"`
	SpeedMPS    float64 `json:"speed_m_s"`
	HeadingDeg  float64 `json:"heading_deg"`
}

// StateAt returns target position at sim time start+duration.
func (m Motion) StateAt(start time.Time, elapsed time.Duration) TargetState {
	distance := m.SpeedMPS * elapsed.Seconds()
	lat, lon := OffsetMeters(m.InitialLat, m.InitialLon, m.HeadingDeg, distance)
	return TargetState{
		Time:      start.Add(elapsed),
		Latitude:  lat,
		Longitude: lon,
		AltitudeM: m.InitialAltM,
	}
}
