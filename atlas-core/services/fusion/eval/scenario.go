package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
)

// Scenario is a fixture used to compare fusion engines offline.
type Scenario struct {
	Name         string                `json:"name"`
	Observations []ScenarioObservation `json:"observations"`
	Expect       Expectation           `json:"expect"`
	GroundTruth  *GroundTruthExpect    `json:"ground_truth,omitempty"`
}

// GroundTruthExpect compares fused track positions to the simulated target end state.
type GroundTruthExpect struct {
	Latitude              float64 `json:"latitude"`
	Longitude             float64 `json:"longitude"`
	ToleranceM            float64 `json:"tolerance_m"`
	MinTracksWithPosition int     `json:"min_tracks_with_position,omitempty"`
}

type ScenarioObservation struct {
	ObservationID     string          `json:"observation_id"`
	SourceAssetID     string          `json:"source_asset_id"`
	TargetEntityID    *string         `json:"target_entity_id,omitempty"`
	LatestTelemetryAt string          `json:"latest_telemetry_at,omitempty"`
	UpdatedAt         string          `json:"updated_at"`
	Version           int             `json:"version"`
	JSON              json.RawMessage `json:"json"`
}

// Expectation describes pass/fail checks on a fusion result.
type Expectation struct {
	TrackUpdates        *int `json:"track_updates,omitempty"`
	ProvenanceRecords   *int `json:"provenance_records,omitempty"`
	ProtocolValidTracks bool `json:"protocol_valid_tracks"`
}

// LoadScenarioDir loads a scenario directory (simulation.json or static scenario.json).
func LoadScenarioDir(dir string) (Scenario, error) {
	_, err := os.Stat(filepath.Join(dir, "simulation.json"))
	if err == nil {
		return MaterializeSimulation(dir)
	}
	if !os.IsNotExist(err) {
		return Scenario{}, fmt.Errorf("stat simulation.json: %w", err)
	}
	return loadStaticScenario(dir)
}

// LoadScenario reads static scenario.json from a scenario directory.
func LoadScenario(dir string) (Scenario, error) {
	return loadStaticScenario(dir)
}

func loadStaticScenario(dir string) (Scenario, error) {
	data, err := os.ReadFile(filepath.Join(dir, "scenario.json"))
	if err != nil {
		return Scenario{}, fmt.Errorf("read scenario.json: %w", err)
	}
	var scenario Scenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return Scenario{}, fmt.Errorf("parse scenario.json: %w", err)
	}
	if scenario.Name == "" {
		scenario.Name = filepath.Base(dir)
	}
	return scenario, nil
}

// ListScenarioDirs returns child directories of root that contain scenario.json.
func ListScenarioDirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read scenarios root: %w", err)
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		ok, err := isScenarioDir(path)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		dirs = append(dirs, path)
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no scenario directories under %s", root)
	}
	return dirs, nil
}

func isScenarioDir(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, "simulation.json"))
	if err == nil {
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat simulation.json: %w", err)
	}
	_, err = os.Stat(filepath.Join(path, "scenario.json"))
	if err == nil {
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat scenario.json: %w", err)
	}
	return false, nil
}

// ObservationBatch builds the fusion input batch for this scenario.
func (s Scenario) ObservationBatch() (core.ObservationBatch, error) {
	return s.observationBatch()
}

func (s Scenario) observationBatch() (core.ObservationBatch, error) {
	inputs := make([]core.ObservationInput, 0, len(s.Observations))
	for i, obs := range s.Observations {
		updatedAt, err := time.Parse(time.RFC3339Nano, obs.UpdatedAt)
		if err != nil {
			return core.ObservationBatch{}, fmt.Errorf("observations[%d].updated_at: %w", i, err)
		}
		input := core.ObservationInput{
			ObservationID:  obs.ObservationID,
			SourceAssetID:  obs.SourceAssetID,
			TargetEntityID: obs.TargetEntityID,
			Version:        obs.Version,
			UpdatedAt:      updatedAt.UTC(),
			JSON:           append([]byte(nil), obs.JSON...),
		}
		if obs.LatestTelemetryAt != "" {
			latest, err := time.Parse(time.RFC3339Nano, obs.LatestTelemetryAt)
			if err != nil {
				return core.ObservationBatch{}, fmt.Errorf("observations[%d].latest_telemetry_at: %w", i, err)
			}
			input.LatestTelemetryAt = latest.UTC()
		}
		inputs = append(inputs, input)
	}
	return core.NewObservationBatch(inputs, core.Checkpoint{}), nil
}
