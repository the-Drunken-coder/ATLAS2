package eval

import (
	"testing"

	"github.com/anomalyco/atlas-core/services/fusion/core"
)

func TestCheckGroundTruthToleranceRequiresPosition(t *testing.T) {
	expect := &GroundTruthExpect{
		Latitude:              40.0,
		Longitude:             -74.0,
		ToleranceM:            100,
		MinTracksWithPosition: 0,
	}
	failures := checkGroundTruth(core.Result{}, expect)
	if len(failures) != 1 {
		t.Fatalf("expected tolerance failure with zero tracks, got %v", failures)
	}
}

func TestCheckGroundTruthMinTracksWithoutTolerance(t *testing.T) {
	expect := &GroundTruthExpect{
		Latitude:              40.0,
		Longitude:             -74.0,
		ToleranceM:            0,
		MinTracksWithPosition: 1,
	}
	failures := checkGroundTruth(core.Result{}, expect)
	if len(failures) != 1 {
		t.Fatalf("expected min-tracks failure with zero tolerance, got %v", failures)
	}
}
