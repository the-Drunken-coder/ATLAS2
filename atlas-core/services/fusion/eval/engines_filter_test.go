package eval

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/anomalyco/atlas-core/services/fusion/engines"
)

func TestFilterEnginesForScenarioRespectsSimulationEngines(t *testing.T) {
	root := testScenariosRootFromCaller(t)
	scenarioDir := filepath.Join(root, "moving_adsb_dual_cam")
	if _, err := LoadSimulation(scenarioDir); err != nil {
		t.Skipf("scenario not present yet: %v", err)
	}

	engineList, err := engines.Resolve([]string{"reference", "multisensor"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	filtered, err := FilterEnginesForScenario(scenarioDir, engineList)
	if err != nil {
		t.Fatalf("FilterEnginesForScenario: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name() != "multisensor" {
		t.Fatalf("expected only multisensor, got %+v", filtered)
	}

	reports, err := RunScenarioDir(context.Background(), scenarioDir, engineList)
	if err != nil {
		t.Fatalf("RunScenarioDir: %v", err)
	}
	if len(reports) != 1 || reports[0].EngineName != "multisensor" {
		t.Fatalf("expected one multisensor report, got %+v", reports)
	}
}

func TestLoadScenarioEngineNamesRejectsUnknownEngine(t *testing.T) {
	dir := t.TempDir()
	simPath := filepath.Join(dir, "simulation.json")
	payload := []byte(`{
  "kind": "simulation",
  "name": "bad_engine",
  "seed": 1,
  "start": "2026-01-01T12:00:00Z",
  "duration_sec": 1,
  "engines": ["multisnesor"],
  "target": {"initial_latitude": 0, "initial_longitude": 0, "initial_altitude_m": 0, "speed_m_s": 0, "heading_deg": 0},
  "feeds": [],
  "expect": {"protocol_valid_tracks": false, "ground_truth_tolerance_m": 0, "min_tracks_with_position": 0}
}`)
	if err := os.WriteFile(simPath, payload, 0o644); err != nil {
		t.Fatalf("write simulation.json: %v", err)
	}
	_, err := LoadScenarioEngineNames(dir)
	if err == nil {
		t.Fatal("expected error for unknown engine name")
	}
}

func TestFilterEnginesForScenarioStaticReferenceOnly(t *testing.T) {
	root := testScenariosRootFromCaller(t)
	scenarioDir := filepath.Join(root, "single_point")

	engineList, err := engines.Resolve([]string{"reference", "multisensor"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	filtered, err := FilterEnginesForScenario(scenarioDir, engineList)
	if err != nil {
		t.Fatalf("FilterEnginesForScenario: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name() != "reference" {
		t.Fatalf("expected only reference, got %+v", filtered)
	}
}

func testScenariosRootFromCaller(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata", "scenarios")
}
