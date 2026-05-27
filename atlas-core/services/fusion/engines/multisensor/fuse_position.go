package multisensor

import (
	"math"

	"github.com/anomalyco/atlas-core/services/fusion/sim"
)

type fusedPosition struct {
	lat         float64
	lon         float64
	altM        *float64
	uncM        float64
	adsbUsed    int
	lobsUsed    int
	sourceCount int
}

func fusePosition(points []pointSample, lobs []lobSample) (fusedPosition, error) {
	var out fusedPosition
	var adsbLat, adsbLon float64
	var adsbAlt *float64
	var adsbUnc float64
	var hasADSB bool
	if p, ok := newestPointSample(points); ok {
		adsbLat, adsbLon = p.lat, p.lon
		adsbAlt = p.altM
		adsbUnc = p.uncertaintyM
		hasADSB = true
		out.adsbUsed = 1
	}

	var lobLat, lobLon float64
	var lobAlt *float64
	lobsUsed := 0
	if lat, lon, alt, used, ok := triangulateLOBs(lobs); ok {
		lobLat, lobLon = lat, lon
		lobAlt = alt
		lobsUsed = used
	}

	hasLOB := lobsUsed > 0
	out.lobsUsed = lobsUsed
	out.sourceCount = out.adsbUsed + out.lobsUsed

	switch {
	case hasADSB && hasLOB:
		out.lat = adsbLat*(1-lobFixWeight) + lobLat*lobFixWeight
		out.lon = sim.BlendLongitudeDegrees(adsbLon, lobLon, lobFixWeight)
		out.altM = altitudeFromADSBAndLOB(adsbAlt, lobAlt)
		out.uncM = adsbUnc
		lobErr := sim.HaversineM(lobLat, lobLon, adsbLat, adsbLon)
		if lobErr > out.uncM {
			out.uncM = lobErr
		}
	case hasADSB:
		out.lat, out.lon = adsbLat, adsbLon
		out.altM = adsbAlt
		out.uncM = adsbUnc
	case hasLOB:
		out.lat, out.lon = lobLat, lobLon
		out.altM = lobAlt
		out.uncM = 200
	default:
		return fusedPosition{}, errNoUsablePosition
	}
	return out, nil
}

type lobTriangulationFix struct {
	lat    float64
	lon    float64
	altM   float64
	weight float64
}

func triangulateLOBs(lobs []lobSample) (lat, lon float64, alt *float64, lobsUsed int, ok bool) {
	if len(lobs) < 2 {
		return 0, 0, nil, 0, false
	}
	if len(lobs) == 2 {
		a, b := lobs[0], lobs[1]
		latT, lonT, altT, _, triOK := sim.TriangulateLOB(
			a.observerLat, a.observerLon, a.observerAltM, a.azimuthDeg, a.elevationDeg,
			b.observerLat, b.observerLon, b.observerAltM, b.azimuthDeg, b.elevationDeg,
		)
		if !triOK {
			return 0, 0, nil, 0, false
		}
		altVal := altT
		return latT, lonT, &altVal, 2, true
	}

	var fixes []lobTriangulationFix
	usedIdx := make(map[int]struct{})
	for i := 0; i < len(lobs); i++ {
		for j := i + 1; j < len(lobs); j++ {
			a, b := lobs[i], lobs[j]
			latT, lonT, altT, sepM, triOK := sim.TriangulateLOB(
				a.observerLat, a.observerLon, a.observerAltM, a.azimuthDeg, a.elevationDeg,
				b.observerLat, b.observerLon, b.observerAltM, b.azimuthDeg, b.elevationDeg,
			)
			if !triOK {
				continue
			}
			usedIdx[i] = struct{}{}
			usedIdx[j] = struct{}{}
			weight := 1 / math.Max(sepM, 1)
			fixes = append(fixes, lobTriangulationFix{
				lat: latT, lon: lonT, altM: altT, weight: weight,
			})
		}
	}
	if len(fixes) == 0 {
		return 0, 0, nil, 0, false
	}
	lat, lon, altM := weightedAverageLOBFix(fixes)
	altVal := altM
	return lat, lon, &altVal, len(usedIdx), true
}

func weightedAverageLOBFix(fixes []lobTriangulationFix) (lat, lon, altM float64) {
	var totalW, latSum, altSum float64
	var x, y float64
	for _, f := range fixes {
		totalW += f.weight
		latSum += f.weight * f.lat
		altSum += f.weight * f.altM
		r := f.lon * math.Pi / 180
		x += f.weight * math.Cos(r)
		y += f.weight * math.Sin(r)
	}
	if totalW == 0 {
		return 0, 0, 0
	}
	lat = latSum / totalW
	lon = math.Atan2(y, x) * 180 / math.Pi
	altM = altSum / totalW
	return lat, lon, altM
}

func altitudeFromADSBAndLOB(adsbAlt, lobAlt *float64) *float64 {
	altM := adsbAlt
	if altM == nil && lobAlt != nil {
		altM = lobAlt
	}
	return altM
}

func fusionConfidence(sourceCount int) float64 {
	switch {
	case sourceCount >= 3:
		return 0.92
	case sourceCount == 2:
		return 0.75
	case sourceCount == 1:
		return 0.5
	default:
		return 0.5
	}
}
