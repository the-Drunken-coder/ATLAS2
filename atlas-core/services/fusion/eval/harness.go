package eval

import (
	"context"
	"fmt"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
)

// EngineRun is the outcome of running one engine on one scenario.
type EngineRun struct {
	EngineName    string   `json:"engine_name"`
	EngineVersion string   `json:"engine_version"`
	TrackUpdates  int      `json:"track_updates"`
	Provenance    int      `json:"provenance_records"`
	Passed        bool     `json:"passed"`
	Failures      []string `json:"failures,omitempty"`
}

// RunEngine fuses a scenario batch and checks expectations.
func RunEngine(ctx context.Context, engine core.Engine, scenario Scenario) (EngineRun, error) {
	batch, err := scenario.ObservationBatch()
	if err != nil {
		return EngineRun{}, err
	}
	result, err := engine.Fuse(ctx, batch)
	if err != nil {
		return EngineRun{}, err
	}
	report := EngineRun{
		EngineName:    engine.Name(),
		EngineVersion: engine.Version(),
		TrackUpdates:  len(result.TrackUpdates),
		Provenance:    len(result.Provenance),
		Passed:        true,
	}
	if scenario.Expect.TrackUpdates != nil && report.TrackUpdates != *scenario.Expect.TrackUpdates {
		report.Failures = append(report.Failures, fmt.Sprintf("track_updates: got %d want %d", report.TrackUpdates, *scenario.Expect.TrackUpdates))
	}
	if scenario.Expect.ProvenanceRecords != nil && report.Provenance != *scenario.Expect.ProvenanceRecords {
		report.Failures = append(report.Failures, fmt.Sprintf("provenance_records: got %d want %d", report.Provenance, *scenario.Expect.ProvenanceRecords))
	}
	if scenario.Expect.ProtocolValidTracks {
		if err := validateTrackJSON(result.TrackUpdates); err != nil {
			report.Failures = append(report.Failures, err.Error())
		}
	}
	report.Failures = append(report.Failures, checkGroundTruth(result, scenario.GroundTruth)...)
	report.Passed = len(report.Failures) == 0
	return report, nil
}

// RunScenarioDir loads a scenario directory and runs each engine, returning one report per engine.
// Engines are intersected with optional per-scenario "engines" in simulation.json or scenario.json.
func RunScenarioDir(ctx context.Context, scenarioDir string, engines []core.Engine) ([]EngineRun, error) {
	filtered, err := FilterEnginesForScenario(scenarioDir, engines)
	if err != nil {
		return nil, err
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	scenario, err := LoadScenarioDir(scenarioDir)
	if err != nil {
		return nil, err
	}
	reports := make([]EngineRun, 0, len(filtered))
	for _, engine := range filtered {
		report, err := RunEngine(ctx, engine, scenario)
		if err != nil {
			return nil, fmt.Errorf("engine %s on %s: %w", engine.Name(), scenario.Name, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// ScenarioReport groups eval results for a single scenario directory.
type ScenarioReport struct {
	Scenario string      `json:"scenario"`
	Runs     []EngineRun `json:"runs"`
}

func validateTrackJSON(updates []core.TrackUpdate) error {
	validator, err := protocolvalidation.New()
	if err != nil {
		return fmt.Errorf("protocol validator: %w", err)
	}
	for i, update := range updates {
		issues := validator.ValidateEntity(&model.Entity{
			EntityID: update.TrackID,
			Type:     model.EntityTypeTrack,
			JSON:     update.JSON,
		})
		if len(issues) > 0 {
			return fmt.Errorf("track_updates[%d] (%s): protocol validation failed: %+v", i, update.TrackID, issues)
		}
	}
	return nil
}
