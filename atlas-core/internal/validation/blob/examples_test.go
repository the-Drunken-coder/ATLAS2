package blob

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/anomalyco/atlas-core/internal/core/model"
)

func TestValidateDocExamples(t *testing.T) {
	// --- Asset examples ---
	assetExamples := loadExamples(t, "../../../../docs/vertical-slice-2/examples/assets.json")

	for name, raw := range assetExamples {
		entity := &model.Entity{EntityID: "test", Type: model.EntityTypeAsset, JSON: raw}
		err := NormalizeEntity(entity, OperationCreate)
		if err != nil {
			var ve *ValidationError
			if ok := asValidationError(err, &ve); ok {
				t.Errorf("Asset %s failed: %v", name, ve.Violations)
			} else {
				t.Errorf("Asset %s failed: %v", name, err)
			}
		} else {
			t.Logf("✅ Asset %s: %s", name, entity.JSON)
		}
	}

	// --- Track examples ---
	trackExamples := loadExamples(t, "../../../../docs/vertical-slice-2/examples/tracks.json")
	for name, raw := range trackExamples {
		entity := &model.Entity{EntityID: "test", Type: model.EntityTypeTrack, JSON: raw}
		err := NormalizeEntity(entity, OperationCreate)
		if err != nil {
			var ve *ValidationError
			if ok := asValidationError(err, &ve); ok {
				t.Errorf("Track %s failed: %v", name, ve.Violations)
			} else {
				t.Errorf("Track %s failed: %v", name, err)
			}
		} else {
			t.Logf("✅ Track %s", name)
		}
	}

	// --- Geofeature examples ---
	gfExamples := loadExamples(t, "../../../../docs/vertical-slice-2/examples/geofeatures.json")
	for name, raw := range gfExamples {
		entity := &model.Entity{EntityID: "test", Type: model.EntityTypeGeofeature, JSON: raw}
		err := NormalizeEntity(entity, OperationCreate)
		if err != nil {
			var ve *ValidationError
			if ok := asValidationError(err, &ve); ok {
				t.Errorf("Geofeature %s failed: %v", name, ve.Violations)
			} else {
				t.Errorf("Geofeature %s failed: %v", name, err)
			}
		} else {
			t.Logf("✅ Geofeature %s", name)
		}
	}

	// --- Task examples ---
	taskExamples := loadExamples(t, "../../../../docs/vertical-slice-2/examples/tasks.json")
	for name, raw := range taskExamples {
		task := &model.Task{TaskID: "test", JSON: raw}
		err := NormalizeTask(task, OperationCreate)
		if err != nil {
			var ve *ValidationError
			if ok := asValidationError(err, &ve); ok {
				t.Errorf("Task %s failed: %v", name, ve.Violations)
			} else {
				t.Errorf("Task %s failed: %v", name, err)
			}
		} else {
			t.Logf("✅ Task %s", name)
		}
	}

	// --- Observation examples ---
	obsExamples := loadExamples(t, "../../../../docs/vertical-slice-2/examples/observations.json")
	for name, raw := range obsExamples {
		obs := &model.Observation{ObservationID: "test", JSON: raw}
		err := NormalizeObservation(obs, OperationCreate)
		if err != nil {
			var ve *ValidationError
			if ok := asValidationError(err, &ve); ok {
				t.Errorf("Observation %s failed: %v", name, ve.Violations)
			} else {
				t.Errorf("Observation %s failed: %v", name, err)
			}
		} else {
			t.Logf("✅ Observation %s", name)
		}
	}
}

func asValidationError(err error, target **ValidationError) bool {
	return errors.As(err, target)
}

func loadExamples(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	var examples map[string]json.RawMessage
	if err := json.Unmarshal(raw, &examples); err != nil {
		t.Fatalf("could not parse %s: %v", path, err)
	}

	return examples
}
