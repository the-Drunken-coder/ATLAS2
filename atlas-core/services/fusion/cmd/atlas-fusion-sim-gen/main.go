package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anomalyco/atlas-core/services/fusion/eval"
)

func main() {
	simDir := flag.String("sim", "", "directory containing simulation.json (required)")
	out := flag.String("out", "", "output scenario.json path (default: <sim-dir>/scenario.generated.json)")
	flag.Parse()
	if *simDir == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas-fusion-sim-gen -sim <dir> [-out <file>]")
		os.Exit(2)
	}
	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(*simDir, "scenario.generated.json")
	}
	if err := eval.WriteMaterializedScenario(*simDir, outPath); err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", outPath)
}
