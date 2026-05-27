package engines

import (
	"testing"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/shared/config"
)

func TestForFusionConfigNilConfig(t *testing.T) {
	engines, err := ForFusionConfig(nil)
	if err != nil {
		t.Fatalf("ForFusionConfig: %v", err)
	}
	if len(engines) != 0 {
		t.Fatalf("expected no engines, got %d", len(engines))
	}
}

func TestForFusionConfigReferenceEnabled(t *testing.T) {
	engines, err := ForFusionConfig(&config.FusionConfig{EnableReferenceEngine: true})
	if err != nil {
		t.Fatalf("ForFusionConfig: %v", err)
	}
	if len(engines) != 1 || engines[0].Name() != "reference" {
		t.Fatalf("expected reference engine, got %+v", engineNames(engines))
	}
}

func TestForFusionConfigReferenceDisabled(t *testing.T) {
	engines, err := ForFusionConfig(&config.FusionConfig{EnableReferenceEngine: false})
	if err != nil {
		t.Fatalf("ForFusionConfig: %v", err)
	}
	if len(engines) != 0 {
		t.Fatalf("expected no engines, got %+v", engineNames(engines))
	}
}

func TestForFusionConfigExplicitEngines(t *testing.T) {
	engines, err := ForFusionConfig(&config.FusionConfig{
		Engines:               []string{"multisensor"},
		EnableReferenceEngine: false,
	})
	if err != nil {
		t.Fatalf("ForFusionConfig: %v", err)
	}
	if len(engines) != 1 || engines[0].Name() != "multisensor" {
		t.Fatalf("expected multisensor engine, got %+v", engineNames(engines))
	}
}

func TestForFusionConfigUnknownEngine(t *testing.T) {
	_, err := ForFusionConfig(&config.FusionConfig{Engines: []string{"experimental"}})
	if err == nil {
		t.Fatal("expected error for unknown engine")
	}
}

func TestResolveMultipleEngines(t *testing.T) {
	engines, err := Resolve([]string{"reference", "multisensor"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(engines) != 2 {
		t.Fatalf("expected two engines, got %+v", engineNames(engines))
	}
}

func engineNames(engines []core.Engine) []string {
	names := make([]string, len(engines))
	for i, engine := range engines {
		names[i] = engine.Name()
	}
	return names
}
