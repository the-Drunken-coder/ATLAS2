package sim

import "math"

const earthRadiusM = 6_371_000

// BearingDegrees returns forward azimuth from observer to target (0=north, 90=east).
func BearingDegrees(obsLat, obsLon, tgtLat, tgtLon float64) float64 {
	φ1 := obsLat * math.Pi / 180
	φ2 := tgtLat * math.Pi / 180
	Δλ := (tgtLon - obsLon) * math.Pi / 180
	y := math.Sin(Δλ) * math.Cos(φ2)
	x := math.Cos(φ1)*math.Sin(φ2) - math.Sin(φ1)*math.Cos(φ2)*math.Cos(Δλ)
	deg := math.Atan2(y, x) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}

// ElevationDegrees returns elevation angle from observer to target.
func ElevationDegrees(obsLat, obsLon, obsAltM, tgtLat, tgtLon, tgtAltM float64) float64 {
	groundM := HaversineM(obsLat, obsLon, tgtLat, tgtLon)
	dAlt := tgtAltM - obsAltM
	return math.Atan2(dAlt, groundM) * 180 / math.Pi
}

// HaversineM returns great-circle distance in meters.
func HaversineM(lat1, lon1, lat2, lon2 float64) float64 {
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	a = math.Max(0, math.Min(1, a))
	return 2 * earthRadiusM * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// OffsetMeters moves a lat/lon by distance along heading (0=north).
func OffsetMeters(lat, lon, headingDeg, distanceM float64) (float64, float64) {
	heading := headingDeg * math.Pi / 180
	dNorth := distanceM * math.Cos(heading)
	dEast := distanceM * math.Sin(heading)
	latRad := lat * math.Pi / 180
	dLat := dNorth / 111_320
	cosLat := math.Cos(latRad)
	var dLon float64
	if math.Abs(cosLat) < 1e-12 {
		dLon = 0
	} else {
		dLon = dEast / (111_320 * cosLat)
	}
	return lat + dLat, lon + dLon
}

// BlendLongitudeDegrees blends two longitudes on the circle; weightOnLon2 is in [0,1].
func BlendLongitudeDegrees(lon1, lon2, weightOnLon2 float64) float64 {
	w1 := 1 - weightOnLon2
	r1 := lon1 * math.Pi / 180
	r2 := lon2 * math.Pi / 180
	x := w1*math.Cos(r1) + weightOnLon2*math.Cos(r2)
	y := w1*math.Sin(r1) + weightOnLon2*math.Sin(r2)
	return math.Atan2(y, x) * 180 / math.Pi
}
