package engines

import (
	"fmt"
	"strings"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/fusion/engines/multisensor"
	"github.com/anomalyco/atlas-core/services/fusion/engines/reference"
	"github.com/anomalyco/atlas-core/services/shared/config"
)

// Resolve returns fusion engines for the given registry names.
func Resolve(names []string) ([]core.Engine, error) {
	var engines []core.Engine
	for _, raw := range names {
		name := strings.TrimSpace(strings.ToLower(raw))
		if name == "" {
			continue
		}
		engine, err := lookup(name)
		if err != nil {
			return nil, err
		}
		engines = append(engines, engine)
	}
	return engines, nil
}

// ForFusionConfig builds the engine list from fusion service configuration.
func ForFusionConfig(cfg *config.FusionConfig) ([]core.Engine, error) {
	if cfg == nil || !cfg.EnableReferenceEngine {
		return nil, nil
	}
	return Resolve([]string{"reference"})
}

// Names returns registered engine names for help text and eval tooling.
func Names() []string {
	return []string{"reference", "multisensor"}
}

func lookup(name string) (core.Engine, error) {
	switch name {
	case "reference":
		return reference.Engine{}, nil
	case "multisensor":
		return multisensor.Engine{}, nil
	default:
		return nil, fmt.Errorf("unknown fusion engine %q (known: %s)", name, strings.Join(Names(), ", "))
	}
}
