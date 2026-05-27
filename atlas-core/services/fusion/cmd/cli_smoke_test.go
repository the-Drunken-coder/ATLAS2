package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAtlasFusionEvalCLI(t *testing.T) {
	root := atlasCoreRoot(t)
	cmd := exec.Command("go", "run", "./services/fusion/cmd/atlas-fusion-eval", "-engines", "reference,multisensor")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("atlas-fusion-eval: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"scenario"`) {
		t.Fatalf("expected JSON scenario reports, got:\n%s", out)
	}
}

func TestAtlasFusionSimGenCLI(t *testing.T) {
	root := atlasCoreRoot(t)
	simDir := filepath.Join(root, "services", "fusion", "testdata", "scenarios", "moving_adsb_dual_cam")
	outPath := filepath.Join(t.TempDir(), "scenario.generated.json")
	cmd := exec.Command(
		"go", "run", "./services/fusion/cmd/atlas-fusion-sim-gen",
		"-sim", simDir,
		"-out", outPath,
	)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("atlas-fusion-sim-gen: %v\n%s", err, out)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected generated scenario at %s: %v", outPath, err)
	}
}

func atlasCoreRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../services/fusion/cmd -> atlas-core
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
