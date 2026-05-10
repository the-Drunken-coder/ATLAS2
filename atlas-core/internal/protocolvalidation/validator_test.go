package protocolvalidation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/anomalyco/atlas-core/internal/core/model"
)

func repoRoot(tb testing.TB) string {
	tb.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		tb.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatal("repo root not found")
		}
		dir = parent
	}
}

func syncProtocol(tb testing.TB) {
	tb.Helper()
	root := repoRoot(tb)
	if _, err := os.Stat(filepath.Join(root, "atlas-core", "protocol", "atlas-protocol-validator.mjs")); err == nil {
		return
	}
	tb.Fatalf("protocol artifacts not synced; run python3 %s protocol-sync", filepath.Join(root, "atlas.py"))
}

func TestRunner_NormalizeEntity_UsesAtlasProtocol(t *testing.T) {
	syncProtocol(t)
	runner := NewRunner()
	entity := &model.Entity{
		EntityID: "track-001",
		Type:     model.EntityTypeTrack,
		JSON:     []byte(`{"components":{"telemetry":{"longitude":-74.0}}}`),
	}
	err := runner.NormalizeEntity(context.Background(), entity, OperationCreate)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected protocol validation error, got %v", err)
	}
	if validationErr.Violations[0].Field != "json.components.telemetry" {
		t.Fatalf("unexpected first violation: %+v", validationErr.Violations)
	}
}

func TestRunner_NormalizeTask_CanonicalizesJSON(t *testing.T) {
	syncProtocol(t)
	runner := NewRunner()
	task := &model.Task{
		TaskID: "task-001",
		JSON:   []byte(`{"components":{"parameters":{},"command":{"type":"move_to_location"}}}`),
	}
	if err := runner.NormalizeTask(context.Background(), task, OperationCreate); err != nil {
		t.Fatalf("NormalizeTask failed: %v", err)
	}
	if got, want := string(task.JSON), `{"components":{"command":{"type":"move_to_location"},"parameters":{}},"extra":{}}`; got != want {
		t.Fatalf("unexpected normalized task JSON\nwant: %s\n got: %s", want, got)
	}
}

func TestRunner_NormalizeObject_CommandCatalogExamplePasses(t *testing.T) {
	syncProtocol(t)
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "atlas-protocol", "examples", "command-catalog.json"))
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	var examples struct {
		Minimum map[string]any `json:"minimum"`
	}
	if err := json.Unmarshal(raw, &examples); err != nil {
		t.Fatalf("unmarshal example: %v", err)
	}
	payload, err := json.Marshal(examples.Minimum)
	if err != nil {
		t.Fatalf("marshal example: %v", err)
	}
	obj := &model.Object{ObjectID: "command_catalog", Type: model.ObjectTypeDocument, JSON: payload}
	if err := NewRunner().NormalizeObject(context.Background(), obj, OperationCreate); err != nil {
		t.Fatalf("NormalizeObject failed: %v", err)
	}
}

func TestRunner_NormalizeObject_RejectsCommandCatalogWithoutCommands(t *testing.T) {
	syncProtocol(t)
	obj := &model.Object{
		ObjectID: "command_catalog",
		Type:     model.ObjectTypeDocument,
		JSON:     []byte(`{}`),
	}
	err := NewRunner().NormalizeObject(context.Background(), obj, OperationCreate)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected protocol validation error, got %v", err)
	}
	if validationErr.Violations[0].Field != "json.commands" {
		t.Fatalf("unexpected first violation: %+v", validationErr.Violations)
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
		t.Fatalf("expected valid schema match, got %v", err)
	}
	if err := ValidateCommandSchema(schema, map[string]any{"mode": "bad", "unexpected": true}); err == nil {
		t.Fatal("expected invalid command parameters")
	}
}
