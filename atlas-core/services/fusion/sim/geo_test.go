package sim

import (
	"math"
	"testing"
)

func TestHaversineMHandlesCoincidentPoints(t *testing.T) {
	dist := HaversineM(40.0, -74.0, 40.0, -74.0)
	if math.IsNaN(dist) {
		t.Fatalf("expected finite distance for coincident points, got NaN")
	}
	if dist != 0 {
		t.Fatalf("expected 0m for coincident points, got %v", dist)
	}
}

func TestOffsetMetersNearEquatorDoesNotExplode(t *testing.T) {
	lat, lon := OffsetMeters(0, 0, 90, 1000)
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		t.Fatalf("expected finite offset near equator, got lat=%v lon=%v", lat, lon)
	}
}

func TestBearingAndElevation(t *testing.T) {
	// Target due east of observer at same altitude.
	az := BearingDegrees(40.0, -74.0, 40.0, -73.99)
	if az < 85 || az > 95 {
		t.Fatalf("expected ~90° bearing, got %.2f", az)
	}
	el := ElevationDegrees(40.0, -74.0, 100, 40.0, -73.99, 100)
	if el < -1 || el > 1 {
		t.Fatalf("expected ~0° elevation, got %.2f", el)
	}
}
