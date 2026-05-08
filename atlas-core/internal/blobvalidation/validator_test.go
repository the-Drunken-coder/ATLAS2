package blobvalidation

import (
	"errors"
	"strings"
	"testing"

	"github.com/anomalyco/atlas-core/internal/model"
)

func TestNormalizeEntity_AssetMinimumAndIdempotent(t *testing.T) {
	entity := &model.Entity{
		EntityID: "asset-1",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`{"components":{"supported_commands":{"commands":[]}}}`),
	}
	if err := NormalizeEntity(entity, OperationCreate); err != nil {
		t.Fatalf("NormalizeEntity failed: %v", err)
	}
	want := `{"components":{"supported_commands":{"commands":[]}},"extra":{}}`
	if string(entity.JSON) != want {
		t.Fatalf("expected canonical json %s, got %s", want, entity.JSON)
	}
	before := string(entity.JSON)
	if err := NormalizeEntity(entity, OperationCreate); err != nil {
		t.Fatalf("second NormalizeEntity failed: %v", err)
	}
	if string(entity.JSON) != before {
		t.Fatalf("expected idempotent normalization, got %s", entity.JSON)
	}
}

func TestNormalizeEntity_RejectsPromotedFieldAndUnknownComponent(t *testing.T) {
	entity := &model.Entity{
		EntityID: "asset-1",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`{"type":"asset","components":{"bogus":{}},"extra":{}}`),
	}
	var validationErr *ValidationError
	if err := NormalizeEntity(entity, OperationCreate); !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if len(validationErr.Violations) < 2 {
		t.Fatalf("expected multiple violations, got %+v", validationErr.Violations)
	}
}

func TestNormalizeTask_RejectsMissingCommandFields(t *testing.T) {
	task := &model.Task{TaskID: "task-1", JSON: []byte(`{"components":{},"extra":{}}`)}
	var validationErr *ValidationError
	if err := NormalizeTask(task, OperationCreate); !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestNormalizeObservation_AllowsLineOfBearingWithoutRange(t *testing.T) {
	obs := &model.Observation{ObservationID: "obs-1", JSON: []byte(`{"state":"active","latest_sighting":{"observed_at":"2026-01-01T00:00:10Z","kind":"line_of_bearing","data":{"azimuth_deg":42.1}},"extra":{}}`)}
	if err := NormalizeObservation(obs, OperationCreate); err != nil {
		t.Fatalf("NormalizeObservation failed: %v", err)
	}
}

func TestNormalizeObject_RejectsReservedManifestKeys(t *testing.T) {
	obj := &model.Object{ObjectID: "obj-1", Type: model.ObjectTypeDocument, JSON: []byte(`{"manifest":{},"extra":{}}`)}
	var validationErr *ValidationError
	if err := NormalizeObject(obj, OperationCreate); !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestNormalizeEntity_RejectsOversizedCustomSection(t *testing.T) {
	payload := `{"components":{"supported_commands":{"commands":[]}},"custom_vendor":{"blob":"` + strings.Repeat("a", maxCustomBlobSize) + `"}}`
	entity := &model.Entity{EntityID: "asset-1", Type: model.EntityTypeAsset, JSON: []byte(payload)}
	if err := NormalizeEntity(entity, OperationCreate); err == nil {
		t.Fatal("expected oversized custom section to fail")
	}
}

func TestValidateCommandSchema(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"latitude": map[string]any{"type": "number"},
			"mode":     map[string]any{"type": "string", "enum": []any{"auto", "manual"}},
		},
		"required":             []any{"latitude"},
		"additionalProperties": false,
	}
	if err := ValidateCommandSchema(schema, map[string]any{"latitude": 40.0, "mode": "auto"}); err != nil {
		t.Fatalf("expected schema validation success, got %v", err)
	}
	if err := ValidateCommandSchema(schema, map[string]any{"mode": "bad"}); err == nil {
		t.Fatal("expected schema validation failure")
	}
}

func TestValidateCommandSchema_RejectsMissingOrUnsupportedType(t *testing.T) {
	tests := []map[string]any{
		{},
		{"type": "funky"},
	}
	for _, schema := range tests {
		if err := ValidateCommandSchema(schema, "value"); err == nil {
			t.Fatalf("expected schema validation failure for schema %#v", schema)
		}
	}
}
