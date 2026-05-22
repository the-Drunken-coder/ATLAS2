package protocolvalidation

import (
	"errors"
	"testing"

	"atlas.local/protocol"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

func mustValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

func TestValidateEntity_RejectsMissingVariant(t *testing.T) {
	v := mustValidator(t)
	entity := &model.Entity{
		EntityID: "ent_001",
		Type:     "",
		JSON:     []byte(`{}`),
	}
	issues := v.ValidateEntity(entity)
	if len(issues) == 0 {
		t.Fatal("expected validation issues for missing variant")
	}
	found := false
	for _, issue := range issues {
		if issue.Code == "invalid_value" && issue.Field == "json" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected invalid_value on field json, got %+v", issues)
	}
}

func TestValidateEntity_AssetWithoutSupportedCommands(t *testing.T) {
	v := mustValidator(t)
	entity := &model.Entity{
		EntityID: "ent_001",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`{}`),
	}
	issues := v.ValidateEntity(entity)
	if len(issues) == 0 {
		t.Fatal("expected validation issues for asset without supported_commands")
	}
}

func TestValidateEntity_ValidAsset(t *testing.T) {
	v := mustValidator(t)
	entity := &model.Entity{
		EntityID: "ent_001",
		Type:     model.EntityTypeAsset,
		JSON:     []byte(`{"components":{"supported_commands":{"commands":["test"]}}}`),
	}
	issues := v.ValidateEntity(entity)
	if len(issues) > 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}

func TestValidateObject_CommandCatalogMappedCorrectly(t *testing.T) {
	v := mustValidator(t)
	obj := &model.Object{
		ObjectID:  "cmd_001",
		Type:      model.ObjectTypeCommandCatalog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{"type":"command_catalog","name":"Test","description":"Test","commands":[]}`),
	}
	issues := v.ValidateObject(obj)
	if len(issues) > 0 {
		t.Fatalf("expected no issues for valid command catalog, got %+v", issues)
	}
}

func TestValidateObject_CommandCatalogRejectsInvalidJSON(t *testing.T) {
	v := mustValidator(t)
	obj := &model.Object{
		ObjectID:  "cmd_001",
		Type:      model.ObjectTypeCommandCatalog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`not json`),
	}
	issues := v.ValidateObject(obj)
	if len(issues) == 0 {
		t.Fatal("expected validation issues for invalid JSON")
	}
}

func TestValidateObject_LogVariant(t *testing.T) {
	v := mustValidator(t)
	obj := &model.Object{
		ObjectID:  "obj_001",
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
	}
	issues := v.ValidateObject(obj)
	if len(issues) > 0 {
		t.Fatalf("expected no issues for minimal log object, got %+v", issues)
	}
}

func TestValidateObject_RejectsEmptyVariant(t *testing.T) {
	v := mustValidator(t)
	obj := &model.Object{
		ObjectID:  "obj_001",
		Type:      "",
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte(`{}`),
	}
	issues := v.ValidateObject(obj)
	if len(issues) == 0 {
		t.Fatal("expected validation issues for empty variant")
	}
	found := false
	for _, issue := range issues {
		if issue.Field == "json" && issue.Code == "invalid_value" && issue.Message == "object variant is required" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected canonical empty-variant issue, got %+v", issues)
	}
}

func TestValidateTask_RejectsMissingCommandType(t *testing.T) {
	v := mustValidator(t)
	task := &model.Task{
		TaskID: "task_001",
		JSON:   []byte(`{}`),
	}
	issues := v.ValidateTask(task)
	if len(issues) == 0 {
		t.Fatal("expected validation issues for task without command.type")
	}
}

func TestValidateTask_Valid(t *testing.T) {
	v := mustValidator(t)
	task := &model.Task{
		TaskID: "task_001",
		JSON:   []byte(`{"components":{"command":{"type":"test_cmd"},"parameters":{}}}`),
	}
	issues := v.ValidateTask(task)
	if len(issues) > 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}

func TestValidateObservation_RejectsState(t *testing.T) {
	v := mustValidator(t)
	obs := &model.Observation{
		ObservationID: "obs_001",
		JSON:          []byte(`{"identity":{"kind":"vehicle"},"state":"active"}`),
	}
	issues := v.ValidateObservation(obs)
	found := false
	for _, issue := range issues {
		if issue.Field == "json.state" && issue.Code == "unknown_field" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unknown_field on json.state, got %+v", issues)
	}
}

func TestValidateObservation_Valid(t *testing.T) {
	v := mustValidator(t)
	obs := &model.Observation{
		ObservationID: "obs_001",
		JSON:          []byte(`{"identity":{"kind":"vehicle"}}`),
	}
	issues := v.ValidateObservation(obs)
	if len(issues) > 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}

func TestValidateObservation_RejectsEmptyObject(t *testing.T) {
	v := mustValidator(t)
	obs := &model.Observation{
		ObservationID: "obs_001",
		JSON:          []byte(`{}`),
	}
	issues := v.ValidateObservation(obs)
	if len(issues) == 0 {
		t.Fatal("expected validation issues for empty observation json")
	}
	found := false
	for _, issue := range issues {
		if issue.Code == "invalid_value" && issue.Field == "json" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestValidateObject_DedicatedHistoryTypes(t *testing.T) {
	v := mustValidator(t)
	for _, tc := range []struct {
		name       string
		objectType model.ObjectType
	}{
		{name: "observation history", objectType: model.ObjectTypeObservationHistory},
		{name: "track provenance", objectType: model.ObjectTypeTrackProvenance},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := &model.Object{
				ObjectID: "obj_001",
				Type:     tc.objectType,
				JSON:     []byte(`{"format_version":"v1"}`),
			}
			issues := v.ValidateObject(obj)
			if len(issues) > 0 {
				t.Fatalf("expected no issues, got %+v", issues)
			}
		})
	}
}

func TestValidationError_ErrorString(t *testing.T) {
	err := NewValidationError([]protocol.ValidationIssue{
		{Field: "json.components.command.type", Code: "required", Message: "command.type is required"},
	})
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatal("expected errors.As to match")
	}
}

func TestValidationError_PreservesIssues(t *testing.T) {
	issues := []protocol.ValidationIssue{
		{Field: "json.a", Code: "required", Message: "a is required"},
		{Field: "json.b", Code: "invalid_value", Message: "b is invalid"},
	}
	err := NewValidationError(issues)
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatal("expected errors.As to match")
	}
	if len(verr.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(verr.Issues))
	}
	if verr.Issues[0].Field != "json.a" {
		t.Fatalf("expected first issue field=json.a, got %s", verr.Issues[0].Field)
	}
}
