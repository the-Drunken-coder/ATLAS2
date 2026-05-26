package main

import (
	"context"
	"encoding/json"
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

	engineList, err := engines.Resolve(strings.Split(*engineNames, ","))
	if err != nil {
		fmt.Fprintf(os.Stderr, "engines: %v\n", err)
		os.Exit(1)
	}
	scenarioDirs, err := eval.ListScenarioDirs(*scenariosRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenarios: %v\n", err)
		os.Exit(1)
	}
	if len(scenarioDirs) == 0 {
		fmt.Fprintf(os.Stderr, "no scenarios found in %s\n", *scenariosRoot)
		os.Exit(1)
	}

	var reports []eval.ScenarioReport
	failed := false
	for _, dir := range scenarioDirs {
		runs, err := eval.RunScenarioDir(context.Background(), dir, engineList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "run %s: %v\n", dir, err)
			os.Exit(1)
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
		candidate := filepath.Join(wd, "services", "fusion", "testdata", "scenarios")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("services", "fusion", "testdata", "scenarios")
}
