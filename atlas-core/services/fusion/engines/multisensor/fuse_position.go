package multisensor

import (
	"github.com/anomalyco/atlas-core/services/fusion/sim"
)

type fusedPosition struct {
	lat         float64
	lon         float64
	altM        *float64
	uncM        float64
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
	}

	var lobLat, lobLon float64
	var lobAlt *float64
	var hasLOB bool
	if lat, lon, alt, ok := triangulateBestLOB(lobs); ok {
		lobLat, lobLon = lat, lon
		lobAlt = alt
		hasLOB = true
	}

	switch {
	case hasADSB && hasLOB:
		out.lat = adsbLat*(1-lobFixWeight) + lobLat*lobFixWeight
		out.lon = adsbLon*(1-lobFixWeight) + lobLon*lobFixWeight
		out.altM = altitudeFromADSBAndLOB(adsbAlt, lobAlt)
		out.uncM = adsbUnc
		lobErr := sim.HaversineM(lobLat, lobLon, adsbLat, adsbLon)
		if lobErr > out.uncM {
			out.uncM = lobErr
		}
		out.sourceCount = 3
	case hasADSB:
		out.lat, out.lon = adsbLat, adsbLon
		out.altM = adsbAlt
		out.uncM = adsbUnc
		out.sourceCount = 1
	case hasLOB:
		out.lat, out.lon = lobLat, lobLon
		out.altM = lobAlt
		out.uncM = 200
		out.sourceCount = 2
	default:
		return fusedPosition{}, errNoUsablePosition
	}
	return out, nil
}

func triangulateBestLOB(lobs []lobSample) (lat, lon float64, alt *float64, ok bool) {
	for i := 0; i < len(lobs); i++ {
		for j := i + 1; j < len(lobs); j++ {
			a, b := lobs[i], lobs[j]
			latT, lonT, altT, triOK := sim.TriangulateLOB(
				a.observerLat, a.observerLon, a.observerAltM, a.azimuthDeg, a.elevationDeg,
				b.observerLat, b.observerLon, b.observerAltM, b.azimuthDeg, b.elevationDeg,
			)
			if triOK {
				altVal := altT
				return latT, lonT, &altVal, true
			}
		}
	}
	return 0, 0, nil, false
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
