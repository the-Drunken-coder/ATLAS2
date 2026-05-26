package sim

import "math"

// TriangulateLOB estimates target position from two observer bearings.
func TriangulateLOB(
	obs1Lat, obs1Lon, obs1AltM, az1, el1 float64,
	obs2Lat, obs2Lon, obs2AltM, az2, el2 float64,
) (lat, lon, altM float64, ok bool) {
	p1 := llhToECEF(obs1Lat, obs1Lon, obs1AltM)
	p2 := llhToECEF(obs2Lat, obs2Lon, obs2AltM)
	d1 := lobDirectionECEF(obs1Lat, obs1Lon, az1, el1)
	d2 := lobDirectionECEF(obs2Lat, obs2Lon, az2, el2)

	c1, c2, parallel := closestPointsOnRays(p1, d1, p2, d2)
	if parallel {
		return 0, 0, 0, false
	}
	mid := vec3Scale(vec3Add(c1, c2), 0.5)
	lat, lon, altM = ecefToLLH(mid)
	return lat, lon, altM, true
}

func lobDirectionECEF(obsLat, obsLon, azimuthDeg, elevationDeg float64) [3]float64 {
	east, north, up := lobUnitVector(azimuthDeg, elevationDeg)
	return enuToECEFDirection(obsLat, obsLon, east, north, up)
}

func lobUnitVector(azimuthDeg, elevationDeg float64) (east, north, up float64) {
	az := azimuthDeg * math.Pi / 180
	el := elevationDeg * math.Pi / 180
	horiz := math.Cos(el)
	return math.Sin(az) * horiz, math.Cos(az) * horiz, math.Sin(el)
}

func enuToECEFDirection(lat, lon, east, north, up float64) [3]float64 {
	latRad := lat * math.Pi / 180
	lonRad := lon * math.Pi / 180
	sinLat, cosLat := math.Sin(latRad), math.Cos(latRad)
	sinLon, cosLon := math.Sin(lonRad), math.Cos(lonRad)
	return [3]float64{
		-sinLon*east - sinLat*cosLon*north + cosLat*cosLon*up,
		cosLon*east - sinLat*sinLon*north + cosLat*sinLon*up,
		cosLat*north + sinLat*up,
	}
}

func llhToECEF(lat, lon, altM float64) [3]float64 {
	latRad := lat * math.Pi / 180
	lonRad := lon * math.Pi / 180
	a := 6378137.0
	f := 1 / 298.257223563
	e2 := 2*f - f*f
	sinLat, cosLat := math.Sin(latRad), math.Cos(latRad)
	sinLon, cosLon := math.Sin(lonRad), math.Cos(lonRad)
	N := a / math.Sqrt(1-e2*sinLat*sinLat)
	return [3]float64{
		(N + altM) * cosLat * cosLon,
		(N + altM) * cosLat * sinLon,
		(N*(1-e2) + altM) * sinLat,
	}
}

func ecefToLLH(p [3]float64) (lat, lon, altM float64) {
	a := 6378137.0
	f := 1 / 298.257223563
	b := a * (1 - f)
	e2 := 2*f - f*f
	ep2 := (a*a - b*b) / (b * b)
	x, y, z := p[0], p[1], p[2]
	lon = math.Atan2(y, x) * 180 / math.Pi
	pDist := math.Hypot(x, y)
	theta := math.Atan2(z*a, pDist*b)
	sinTheta, cosTheta := math.Sin(theta), math.Cos(theta)
	lat = math.Atan2(z+ep2*b*sinTheta*sinTheta*sinTheta, pDist-a*e2*cosTheta*cosTheta*cosTheta) * 180 / math.Pi
	latRad := lat * math.Pi / 180
	N := a / math.Sqrt(1-e2*math.Sin(latRad)*math.Sin(latRad))
	altM = pDist/math.Cos(latRad) - N
	return lat, lon, altM
}

func closestPointsOnRays(p1, d1, p2, d2 [3]float64) (c1, c2 [3]float64, parallel bool) {
	w0 := vec3Sub(p1, p2)
	a := vec3Dot(d1, d1)
	b := vec3Dot(d1, d2)
	c := vec3Dot(d2, d2)
	d := vec3Dot(d1, w0)
	e := vec3Dot(d2, w0)
	denom := a*c - b*b
	if math.Abs(denom) < 1e-9 {
		return c1, c2, true
	}
	t1 := (b*e - c*d) / denom
	t2 := (a*e - b*d) / denom
	if t1 < 0 {
		t1 = 0
	}
	if t2 < 0 {
		t2 = 0
	}
	return vec3Add(p1, vec3Scale(d1, t1)), vec3Add(p2, vec3Scale(d2, t2)), false
}

func vec3Add(a, b [3]float64) [3]float64 {
	return [3]float64{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

func vec3Sub(a, b [3]float64) [3]float64 {
	return [3]float64{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

func vec3Scale(a [3]float64, s float64) [3]float64 {
	return [3]float64{a[0] * s, a[1] * s, a[2] * s}
}

func vec3Dot(a, b [3]float64) float64 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}
