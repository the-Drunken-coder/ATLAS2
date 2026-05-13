package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type validManifest struct {
	Cases []validCase `json:"cases"`
}

type validCase struct {
	ID       string       `json:"id"`
	Resource ResourceKind `json:"resource"`
	Source   string       `json:"source"`
	Example  string       `json:"example"`
	Variant  string       `json:"variant,omitempty"`
}

type invalidManifest struct {
	Cases []invalidCase `json:"cases"`
}

type invalidCase struct {
	ID       string            `json:"id"`
	Resource ResourceKind      `json:"resource"`
	Source   string            `json:"source"`
	Variant  string            `json:"variant,omitempty"`
	Expected []ValidationIssue `json:"expected"`
}

func TestValidExamples(t *testing.T) {
	root := protocolRoot(t)
	validator, err := NewWithProtocolRoot(root)
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	var manifest validManifest
	readJSON(t, filepath.Join(root, "source", "manifests", "valid-examples.json"), &manifest)
	for _, tc := range manifest.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			var wrapped map[string]json.RawMessage
			readJSON(t, filepath.Join(root, tc.Source), &wrapped)
			payload, ok := wrapped[tc.Example]
			if !ok {
				t.Fatalf("missing example %s", tc.Example)
			}
			issues := validator.ValidateBytes(tc.Resource, payload, WithVariant(tc.Variant))
			if len(issues) != 0 {
				t.Fatalf("expected valid example, got %#v", issues)
			}
		})
	}
}

func TestInvalidGoldens(t *testing.T) {
	root := protocolRoot(t)
	validator, err := NewWithProtocolRoot(root)
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	var manifest invalidManifest
	readJSON(t, filepath.Join(root, "source", "manifests", "invalid-cases.json"), &manifest)
	for _, tc := range manifest.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			payload, err := os.ReadFile(filepath.Join(root, tc.Source))
			if err != nil {
				t.Fatalf("read payload: %v", err)
			}
			actual := NormalizeValidationIssues(validator.ValidateBytes(tc.Resource, payload, WithVariant(tc.Variant)))
			expected := NormalizeValidationIssues(tc.Expected)
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("issues mismatch\nexpected=%#v\nactual=%#v", expected, actual)
			}
		})
	}
}

func protocolRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(cwd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "source", "schemas")); err != nil {
		t.Fatalf("resolve protocol root: %v", err)
	}
	return root
}

func readJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}
