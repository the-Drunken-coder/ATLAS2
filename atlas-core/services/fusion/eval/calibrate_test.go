package eval

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/anomalyco/atlas-core/services/fusion/engines"
	"github.com/anomalyco/atlas-core/services/fusion/sim"
)

// TestReportScenarioGroundTruthErrors prints fused vs ground-truth distance per scenario.
// Run with: FUSION_CALIBRATE=1 go test ./eval -run TestReportScenarioGroundTruthErrors -v
func TestReportScenarioGroundTruthErrors(t *testing.T) {
	if os.Getenv("FUSION_CALIBRATE") == "" {
		t.Skip("set FUSION_CALIBRATE=1 to run")
	}
	root := scenariosRoot(t)
	dirs, err := ListScenarioDirs(root)
	if err != nil {
		t.Fatalf("ListScenarioDirs: %v", err)
	}
	for _, engineName := range []string{"reference", "multisensor"} {
		engineList, err := engines.Resolve([]string{engineName})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		engine := engineList[0]
		for _, dir := range dirs {
			filtered, err := FilterEnginesForScenario(dir, engineList)
			if err != nil {
				if errors.Is(err, ErrNoMatchingEngines) {
					continue
				}
				t.Fatalf("FilterEnginesForScenario %s: %v", dir, err)
			}
			if len(filtered) == 0 {
				continue
			}
			scenario, err := LoadScenarioDir(dir)
			if err != nil {
				t.Fatalf("load %s: %v", dir, err)
			}
			if scenario.GroundTruth == nil || scenario.GroundTruth.ToleranceM <= 0 {
				continue
			}
			batch, err := scenario.ObservationBatch()
			if err != nil {
				t.Fatalf("batch %s: %v", dir, err)
			}
			result, err := engine.Fuse(context.Background(), batch)
			if err != nil {
				t.Fatalf("fuse %s: %v", dir, err)
			}
			if len(result.TrackUpdates) == 0 {
				continue
			}
			bestErr := math.MaxFloat64
			found := false
			for _, update := range result.TrackUpdates {
				lat, lon, ok := trackPosition(update.JSON)
				if !ok {
					continue
				}
				d := sim.HaversineM(lat, lon, scenario.GroundTruth.Latitude, scenario.GroundTruth.Longitude)
				if d < bestErr {
					bestErr = d
					found = true
				}
			}
			if !found {
				continue
			}
			t.Logf("%s %s: %.2fm (tol %.0fm)", engineName, filepath.Base(dir), bestErr, scenario.GroundTruth.ToleranceM)
		}
	}
}
