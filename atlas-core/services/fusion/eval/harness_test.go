package eval

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/anomalyco/atlas-core/services/fusion/engines"
)

func testScenariosRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata", "scenarios")
}

func TestMultisensorEnginePassesDualCameraScenario(t *testing.T) {
	engineList, err := engines.Resolve([]string{"multisensor"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	scenarioDir := filepath.Join(testScenariosRoot(t), "moving_adsb_dual_cam")
	_, reports, err := RunScenarioDir(context.Background(), scenarioDir, engineList)
	if err != nil {
		t.Fatalf("RunScenarioDir failed: %v", err)
	}
	if len(reports) != 1 || !reports[0].Passed {
		t.Fatalf("expected passing multisensor scenario, got %+v", reports)
	}
	if reports[0].TrackUpdates != 1 || reports[0].Provenance != 1 {
		t.Fatalf("unexpected counts: %+v", reports[0])
	}
}

func TestReferenceEnginePassesSimulatedMovingTargetScenario(t *testing.T) {
	engineList, err := engines.Resolve([]string{"reference"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	scenarioDir := filepath.Join(testScenariosRoot(t), "moving_adsb_single_cam")
	_, reports, err := RunScenarioDir(context.Background(), scenarioDir, engineList)
	if err != nil {
		t.Fatalf("RunScenarioDir failed: %v", err)
	}
	if len(reports) != 1 || !reports[0].Passed {
		t.Fatalf("expected passing simulated scenario, got %+v", reports)
	}
}

func TestReferenceEnginePassesSinglePointScenario(t *testing.T) {
	engineList, err := engines.Resolve([]string{"reference"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	scenarioDir := filepath.Join(testScenariosRoot(t), "single_point")
	_, reports, err := RunScenarioDir(context.Background(), scenarioDir, engineList)
	if err != nil {
		t.Fatalf("RunScenarioDir failed: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected one engine report, got %+v", reports)
	}
	if !reports[0].Passed {
		t.Fatalf("scenario failed: %+v", reports[0])
	}
}
