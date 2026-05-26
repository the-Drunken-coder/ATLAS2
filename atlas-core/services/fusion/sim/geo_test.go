package sim

import "testing"

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
