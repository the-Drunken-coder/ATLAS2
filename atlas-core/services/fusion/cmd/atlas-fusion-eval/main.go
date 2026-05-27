package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anomalyco/atlas-core/services/fusion/engines"
	"github.com/anomalyco/atlas-core/services/fusion/eval"
)

func main() {
	scenariosRoot := flag.String("scenarios", defaultScenariosRoot(), "directory containing scenario subdirectories")
	engineNames := flag.String("engines", "reference", "comma-separated fusion engine names")
	flag.Parse()

	rawNames := strings.Split(*engineNames, ",")
	names := make([]string, 0, len(rawNames))
	for _, n := range rawNames {
		n = strings.TrimSpace(n)
		if n != "" {
			names = append(names, n)
		}
	}
	engineList, err := engines.Resolve(names)
	if err != nil {
		fmt.Fprintf(os.Stderr, "engines: %v\n", err)
		os.Exit(1)
	}
	scenarioDirs, err := eval.ListScenarioDirs(*scenariosRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenarios: %v\n", err)
		os.Exit(1)
	}

	var reports []eval.ScenarioReport
	failed := false
	for _, dir := range scenarioDirs {
		runs, err := eval.RunScenarioDir(context.Background(), dir, engineList)
		if err != nil {
			if errors.Is(err, eval.ErrNoMatchingEngines) {
				logSkippedScenario(dir, *engineNames, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "run %s: %v\n", dir, err)
			os.Exit(1)
		}
		if len(runs) == 0 {
			continue
		}
		scenario, err := eval.LoadScenarioDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load %s: %v\n", dir, err)
			os.Exit(1)
		}
		for _, run := range runs {
			if !run.Passed {
				failed = true
			}
		}
		reports = append(reports, eval.ScenarioReport{
			Scenario: scenario.Name,
			Runs:     runs,
		})
	}

	if len(reports) == 0 && len(scenarioDirs) > 0 {
		fmt.Fprintf(os.Stderr, "no scenarios ran for engines %q; check per-scenario engines filters\n", *engineNames)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(reports); err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
		os.Exit(1)
	}
	if failed {
		os.Exit(1)
	}
}

func defaultScenariosRoot() string {
	if wd, err := os.Getwd(); err == nil {
		for _, candidate := range []string{
			filepath.Join(wd, "testdata", "scenarios"),
			filepath.Join(wd, "services", "fusion", "testdata", "scenarios"),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return filepath.Join("testdata", "scenarios")
}

func logSkippedScenario(dir, cliEngines string, err error) {
	name := filepath.Base(dir)
	var mismatch *eval.EngineMismatchError
	if errors.As(err, &mismatch) {
		fmt.Fprintf(os.Stderr, "skip %s: scenario engines %v do not intersect CLI %q\n", name, mismatch.Allowed, cliEngines)
		return
	}
	fmt.Fprintf(os.Stderr, "skip %s: no matching engines for CLI %q\n", name, cliEngines)
}
