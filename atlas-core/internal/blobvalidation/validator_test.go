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

func TestNormalizeEntity_TrackRequiresTelemetryWithPosition(t *testing.T) {
	entity := &model.Entity{EntityID: "track-1", Type: model.EntityTypeTrack, JSON: []byte(`{"components":{"telemetry":{"speed_m_s":4.0}},"extra":{}}`)}
	var validationErr *ValidationError
	if err := NormalizeEntity(entity, OperationCreate); !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	// telemetry present but missing latitude and longitude (requirePosition=true)
	found := false
	for _, v := range validationErr.Violations {
		if v.Field == "json.components.telemetry" && v.Code == "REQUIRED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected REQUIRED violation for telemetry lat/lon, got %+v", validationErr.Violations)
	}
}

func TestNormalizeEntity_GeofeatureRequiresGeometry(t *testing.T) {
	entity := &model.Entity{EntityID: "gf-1", Type: model.EntityTypeGeofeature, JSON: []byte(`{"components":{},"extra":{}}`)}
	var validationErr *ValidationError
	if err := NormalizeEntity(entity, OperationCreate); !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	found := false
	for _, v := range validationErr.Violations {
		if v.Field == "json.components.geometry" && v.Code == "REQUIRED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected REQUIRED violation for geometry, got %+v", validationErr.Violations)
	}
}

func TestNormalizeEntity_TelemetryLatLonRange(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantField string
		wantCode  string
	}{
		{"latitude out of range", `{"components":{"supported_commands":{"commands":[]},"telemetry":{"latitude":100,"longitude":0}},"extra":{}}`, "json.components.telemetry.latitude", "OUT_OF_RANGE"},
		{"longitude out of range", `{"components":{"supported_commands":{"commands":[]},"telemetry":{"latitude":0,"longitude":200}},"extra":{}}`, "json.components.telemetry.longitude", "OUT_OF_RANGE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{EntityID: "asset-1", Type: model.EntityTypeAsset, JSON: []byte(tt.json)}
			var validationErr *ValidationError
			if err := NormalizeEntity(entity, OperationCreate); !errors.As(err, &validationErr) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			found := false
			for _, v := range validationErr.Violations {
				if v.Field == tt.wantField && v.Code == tt.wantCode {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected %s %s, got %+v", tt.wantCode, tt.wantField, validationErr.Violations)
			}
		})
	}
}

func TestNormalizeObservation_ValidStates(t *testing.T) {
	tests := []string{"active", "inactive", "ended"}
	for _, state := range tests {
		obs := &model.Observation{ObservationID: "obs-1", JSON: []byte(`{"state":"` + state + `"}`)}
		if err := NormalizeObservation(obs, OperationCreate); err != nil {
			t.Fatalf("expected state %q to be valid, got %v", state, err)
		}
	}
}

func TestNormalizeObservation_RejectsInvalidState(t *testing.T) {
	obs := &model.Observation{ObservationID: "obs-1", JSON: []byte(`{"state":"invalid_state"}`)}
	var validationErr *ValidationError
	if err := NormalizeObservation(obs, OperationCreate); !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	found := false
	for _, v := range validationErr.Violations {
		if v.Field == "json.state" && v.Code == "INVALID_VALUE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected INVALID_VALUE for state, got %+v", validationErr.Violations)
	}
}

func TestNormalizeTask_Idempotent(t *testing.T) {
	task := &model.Task{TaskID: "task-1", JSON: []byte(`{"components":{"command":{"type":"move_to_location"},"parameters":{}}}`)}
	if err := NormalizeTask(task, OperationCreate); err != nil {
		t.Fatalf("NormalizeTask failed: %v", err)
	}
	before := string(task.JSON)
	if err := NormalizeTask(task, OperationCreate); err != nil {
		t.Fatalf("second NormalizeTask failed: %v", err)
	}
	if string(task.JSON) != before {
		t.Fatalf("expected idempotent normalization, got %s (was %s)", task.JSON, before)
	}
}

func TestNormalizeObservation_Idempotent(t *testing.T) {
	obs := &model.Observation{ObservationID: "obs-1", JSON: []byte(`{"state":"active"}`)}
	if err := NormalizeObservation(obs, OperationCreate); err != nil {
		t.Fatalf("NormalizeObservation failed: %v", err)
	}
	before := string(obs.JSON)
	if err := NormalizeObservation(obs, OperationCreate); err != nil {
		t.Fatalf("second NormalizeObservation failed: %v", err)
	}
	if string(obs.JSON) != before {
		t.Fatalf("expected idempotent normalization, got %s (was %s)", obs.JSON, before)
	}
}

func TestNormalizeObject_Idempotent(t *testing.T) {
	obj := &model.Object{ObjectID: "obj-1", Type: model.ObjectTypeLog, JSON: []byte(`{"log_type":"system"}`)}
	if err := NormalizeObject(obj, OperationCreate); err != nil {
		t.Fatalf("NormalizeObject failed: %v", err)
	}
	before := string(obj.JSON)
	if err := NormalizeObject(obj, OperationCreate); err != nil {
		t.Fatalf("second NormalizeObject failed: %v", err)
	}
	if string(obj.JSON) != before {
		t.Fatalf("expected idempotent normalization, got %s (was %s)", obj.JSON, before)
	}
}

func TestNormalizeObject_RejectsPromotedField(t *testing.T) {
	obj := &model.Object{ObjectID: "obj-1", Type: model.ObjectTypeDocument, JSON: []byte(`{"owner_type":"system","extra":{}}`)}
	var validationErr *ValidationError
	if err := NormalizeObject(obj, OperationCreate); !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	found := false
	for _, v := range validationErr.Violations {
		if v.Field == "json.owner_type" && v.Code == "PROMOTED_FIELD" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected PROMOTED_FIELD violation, got %+v", validationErr.Violations)
	}
}

func TestNormalizeEntity_RejectsOversizedCustomSection(t *testing.T) {
	payload := `{"components":{"supported_commands":{"commands":[]}},"custom_vendor":{"blob":"` + strings.Repeat("a", maxCustomBlobSize) + `"}}`
	entity := &model.Entity{EntityID: "asset-1", Type: model.EntityTypeAsset, JSON: []byte(payload)}
	if err := NormalizeEntity(entity, OperationCreate); err == nil {
		t.Fatal("expected oversized custom section to fail")
	}
}

func TestNormalizeEntity_SupportedCommandsWrongTypeDoesNotAlsoReportRequired(t *testing.T) {
	entity := &model.Entity{
		EntityID: "asset-1",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`{"components":{"supported_commands":{"commands":"fire"}},"extra":{}}`),
	}

	var validationErr *ValidationError
	if err := NormalizeEntity(entity, OperationCreate); !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}

	foundInvalidType := false
	for _, violation := range validationErr.Violations {
		if violation.Field == "json.components.supported_commands.commands" && violation.Code == "INVALID_TYPE" {
			foundInvalidType = true
		}
		if violation.Field == "json.components.supported_commands.commands" && violation.Code == "REQUIRED" {
			t.Fatalf("did not expect REQUIRED violation when commands is present: %+v", validationErr.Violations)
		}
	}
	if !foundInvalidType {
		t.Fatalf("expected INVALID_TYPE violation, got %+v", validationErr.Violations)
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
