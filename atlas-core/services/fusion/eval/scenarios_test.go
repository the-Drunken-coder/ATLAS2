package eval

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/anomalyco/atlas-core/services/fusion/engines"
)

func scenariosRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata", "scenarios")
}

func TestAllScenariosReferenceEngine(t *testing.T) {
	runScenariosForEngine(t, "reference")
}

func TestAllScenariosMultisensorEngine(t *testing.T) {
	runScenariosForEngine(t, "multisensor")
}

func runScenariosForEngine(t *testing.T, engineName string) {
	t.Helper()
	root := scenariosRoot(t)
	dirs, err := ListScenarioDirs(root)
	if err != nil {
		t.Fatalf("ListScenarioDirs: %v", err)
	}
	engineList, err := engines.Resolve([]string{engineName})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	var ran int
	for _, dir := range dirs {
		dir := dir
		t.Run(filepath.Base(dir), func(t *testing.T) {
			filtered, err := FilterEnginesForScenario(dir, engineList)
			if err != nil {
				t.Fatalf("FilterEnginesForScenario %s: %v", dir, err)
			}
			if len(filtered) == 0 {
				t.Skip("engine filtered out for scenario")
			}
			reports, err := RunScenarioDir(context.Background(), dir, engineList)
			if err != nil {
				t.Fatalf("RunScenarioDir %s: %v", filepath.Base(dir), err)
			}
			if len(reports) != 1 {
				t.Fatalf("%s: expected one report, got %d", filepath.Base(dir), len(reports))
			}
			if reports[0].EngineName != engineName {
				t.Fatalf("%s: expected engine %s, got %s", filepath.Base(dir), engineName, reports[0].EngineName)
			}
			if !reports[0].Passed {
				t.Fatalf("%s: scenario failed: %+v", filepath.Base(dir), reports[0])
			}
			ran++
		})
	}
	if ran == 0 {
		t.Fatalf("no scenarios ran for engine %s", engineName)
	}
}
