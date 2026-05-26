package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anomalyco/atlas-core/services/fusion/core"
)

// LoadScenarioEngineNames returns engine names declared for a scenario directory.
// When nil or empty, all CLI engines apply.
func LoadScenarioEngineNames(scenarioDir string) ([]string, error) {
	simPath := filepath.Join(scenarioDir, "simulation.json")
	if _, statErr := os.Stat(simPath); statErr == nil {
		file, err := LoadSimulation(scenarioDir)
		if err != nil {
			return nil, err
		}
		return normalizeEngineNames(file.Engines), nil
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("stat simulation.json: %w", statErr)
	}

	scenario, err := loadStaticScenario(scenarioDir)
	if err != nil {
		return nil, err
	}
	return normalizeEngineNames(scenario.Engines), nil
}

// FilterEnginesForScenario intersects CLI engines with scenario-declared engines.
func FilterEnginesForScenario(scenarioDir string, engines []core.Engine) ([]core.Engine, error) {
	allowed, err := LoadScenarioEngineNames(scenarioDir)
	if err != nil {
		return nil, err
	}
	if len(allowed) == 0 {
		return engines, nil
	}
	allowSet := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		allowSet[name] = struct{}{}
	}
	var filtered []core.Engine
	for _, engine := range engines {
		if _, ok := allowSet[strings.ToLower(engine.Name())]; ok {
			filtered = append(filtered, engine)
		}
	}
	return filtered, nil
}

func normalizeEngineNames(names []string) []string {
	var out []string
	for _, raw := range names {
		name := strings.TrimSpace(strings.ToLower(raw))
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}
