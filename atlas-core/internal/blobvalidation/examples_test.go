package blobvalidation

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/anomalyco/atlas-core/internal/model"
)

func TestValidateDocExamples(t *testing.T) {
	// --- Asset examples ---
	assetRaw, _ := os.ReadFile("../../docs/vertical-slice-2/examples/assets.json")
	var assetExamples map[string]json.RawMessage
	json.Unmarshal(assetRaw, &assetExamples)

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
	trackRaw, _ := os.ReadFile("../../docs/vertical-slice-2/examples/tracks.json")
	var trackExamples map[string]json.RawMessage
	json.Unmarshal(trackRaw, &trackExamples)
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
	gfRaw, _ := os.ReadFile("../../docs/vertical-slice-2/examples/geofeatures.json")
	var gfExamples map[string]json.RawMessage
	json.Unmarshal(gfRaw, &gfExamples)
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
	taskRaw, _ := os.ReadFile("../../docs/vertical-slice-2/examples/tasks.json")
	var taskExamples map[string]json.RawMessage
	json.Unmarshal(taskRaw, &taskExamples)
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
	obsRaw, _ := os.ReadFile("../../docs/vertical-slice-2/examples/observations.json")
	var obsExamples map[string]json.RawMessage
	json.Unmarshal(obsRaw, &obsExamples)
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

	fmt.Println("examples test complete")
}

func asValidationError(err error, target **ValidationError) bool {
	if ve, ok := err.(*ValidationError); ok {
		*target = ve
		return true
	}
	return false
}
