package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMaterializeSimulationPreservesEngines(t *testing.T) {
	root := testScenariosRootFromSimulationTest(t)
	dir := filepath.Join(root, "moving_adsb_dual_cam")
	if _, err := os.Stat(filepath.Join(dir, "simulation.json")); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("scenario not present: %v", err)
		}
		t.Fatalf("stat simulation.json: %v", err)
	}

	scenario, err := MaterializeSimulation(dir)
	if err != nil {
		t.Fatalf("MaterializeSimulation: %v", err)
	}
	if len(scenario.Engines) != 1 || scenario.Engines[0] != "multisensor" {
		t.Fatalf("expected engines [multisensor], got %v", scenario.Engines)
	}
}

func TestWriteMaterializedScenarioPreservesEngines(t *testing.T) {
	root := testScenariosRootFromSimulationTest(t)
	dir := filepath.Join(root, "moving_adsb_dual_cam")
	if _, err := os.Stat(filepath.Join(dir, "simulation.json")); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("scenario not present: %v", err)
		}
		t.Fatalf("stat simulation.json: %v", err)
	}

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "scenario.generated.json")
	if err := WriteMaterializedScenario(dir, outPath); err != nil {
		t.Fatalf("WriteMaterializedScenario: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read generated scenario: %v", err)
	}
	var payload struct {
		Engines []string `json:"engines"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("parse generated scenario: %v", err)
	}
	if len(payload.Engines) != 1 || payload.Engines[0] != "multisensor" {
		t.Fatalf("expected engines [multisensor] in export, got %v", payload.Engines)
	}
}

func testScenariosRootFromSimulationTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata", "scenarios")
}
