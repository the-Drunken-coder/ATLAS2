package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/sim"
)

// SimulationFile is the on-disk format for target-centric scenarios.
type SimulationFile struct {
	Kind        string           `json:"kind"`
	Name        string           `json:"name"`
	Seed        int64            `json:"seed"`
	Start       string           `json:"start"`
	DurationSec float64          `json:"duration_sec"`
	Target      sim.Motion       `json:"target"`
	Feeds       []sim.FeedConfig `json:"feeds"`
	Expect      SimulationExpect `json:"expect"`
	Engines     []string         `json:"engines,omitempty"`
}

// SimulationExpect validates fusion output against simulated ground truth.
type SimulationExpect struct {
	ProtocolValidTracks   bool    `json:"protocol_valid_tracks"`
	GroundTruthToleranceM float64 `json:"ground_truth_tolerance_m"`
	MinTracksWithPosition int     `json:"min_tracks_with_position"`
	TrackUpdates          *int    `json:"track_updates,omitempty"`
	ProvenanceRecords     *int    `json:"provenance_records,omitempty"`
}

// LoadSimulation reads simulation.json from dir.
func LoadSimulation(dir string) (SimulationFile, error) {
	data, err := os.ReadFile(filepath.Join(dir, "simulation.json"))
	if err != nil {
		return SimulationFile{}, fmt.Errorf("read simulation.json: %w", err)
	}
	var file SimulationFile
	if err := json.Unmarshal(data, &file); err != nil {
		return SimulationFile{}, fmt.Errorf("parse simulation.json: %w", err)
	}
	if file.Kind != "" && file.Kind != "simulation" {
		return SimulationFile{}, fmt.Errorf("unsupported scenario kind %q", file.Kind)
	}
	if file.Name == "" {
		file.Name = filepath.Base(dir)
	}
	return file, nil
}

// MaterializeSimulation runs the world model and builds an eval Scenario.
func MaterializeSimulation(dir string) (Scenario, error) {
	file, err := LoadSimulation(dir)
	if err != nil {
		return Scenario{}, err
	}
	def := sim.Definition{
		Name:         file.Name,
		Seed:         file.Seed,
		StartRFC3339: file.Start,
		DurationSec:  file.DurationSec,
		Target:       file.Target,
		Feeds:        file.Feeds,
	}
	result, err := sim.Run(def)
	if err != nil {
		return Scenario{}, fmt.Errorf("run simulation: %w", err)
	}

	scenario := Scenario{
		Name:    file.Name,
		Engines: normalizeEngineNames(file.Engines),
		Expect: Expectation{
			ProtocolValidTracks: file.Expect.ProtocolValidTracks,
			TrackUpdates:        file.Expect.TrackUpdates,
			ProvenanceRecords:   file.Expect.ProvenanceRecords,
		},
		GroundTruth: &GroundTruthExpect{
			Latitude:              result.GroundTruth.Latitude,
			Longitude:             result.GroundTruth.Longitude,
			ToleranceM:            file.Expect.GroundTruthToleranceM,
			MinTracksWithPosition: file.Expect.MinTracksWithPosition,
		},
	}
	for _, snap := range result.Observations {
		scenario.Observations = append(scenario.Observations, ScenarioObservation{
			ObservationID:     snap.ObservationID,
			SourceAssetID:     snap.SourceAssetID,
			LatestTelemetryAt: snap.LatestTelemetryAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:         snap.UpdatedAt.UTC().Format(time.RFC3339Nano),
			Version:           snap.Version,
			JSON:              append([]byte(nil), snap.JSON...),
		})
	}
	return scenario, nil
}

// WriteMaterializedScenario runs simulation.json and writes generated scenario.json (debug/export).
func WriteMaterializedScenario(simDir, outPath string) error {
	scenario, err := MaterializeSimulation(simDir)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"name":         scenario.Name,
		"generated":    true,
		"observations": scenario.Observations,
		"expect":       scenario.Expect,
	}
	if len(scenario.Engines) > 0 {
		payload["engines"] = scenario.Engines
	}
	if scenario.GroundTruth != nil {
		payload["ground_truth"] = scenario.GroundTruth
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(outPath, data, 0o644)
}
