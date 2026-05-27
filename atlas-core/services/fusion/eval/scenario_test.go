package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListScenarioDirsPropagatesStatErrors(t *testing.T) {
	root := t.TempDir()
	badDir := filepath.Join(root, "broken")
	if err := os.Mkdir(badDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(badDir, 0o000); err != nil {
		t.Skipf("chmod not supported: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(badDir, 0o755) })

	_, err := ListScenarioDirs(root)
	if err == nil {
		t.Fatal("expected stat error when scenario directory is not accessible")
	}
}

func TestIsScenarioDirRecognizesSimulationJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "simulation.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write simulation.json: %v", err)
	}
	ok, err := isScenarioDir(dir)
	if err != nil {
		t.Fatalf("isScenarioDir: %v", err)
	}
	if !ok {
		t.Fatal("expected directory with simulation.json to be a scenario dir")
	}
}

func TestIsScenarioDirRecognizesScenarioJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "scenario.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatalf("write scenario.json: %v", err)
	}
	ok, err := isScenarioDir(dir)
	if err != nil {
		t.Fatalf("isScenarioDir: %v", err)
	}
	if !ok {
		t.Fatal("expected directory with scenario.json to be a scenario dir")
	}
}
