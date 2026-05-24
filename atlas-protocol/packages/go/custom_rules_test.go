package protocol

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestForbiddenObservationFieldsRejectExplicitNull(t *testing.T) {
	for _, rejected := range []string{"state", "latest_sighting", "sightings_object_id"} {
		payload, err := json.Marshal(map[string]any{rejected: nil})
		if err != nil {
			t.Fatalf("marshal %s: %v", rejected, err)
		}
		issues := validateObservationPre(mustObject(t, payload))
		if !hasIssue(issues, "json."+rejected, "unknown_field") {
			t.Fatalf("expected unknown_field for json.%s with null, got %#v", rejected, issues)
		}
	}
}

func TestObservationHistoryEventSizeLimitUsesRawPayloadBytes(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	payload := append(append([]byte("{"), bytes.Repeat([]byte{' '}, rootMaxBytes+1)...), []byte(`"event_id":"evt-1","event_type":"telemetry","recorded_at":"2026-01-01T00:00:00Z","observation_id":"obs-1","base_observation_version":1,"observed_at":"2026-01-01T00:00:00Z","payload":{"kind":"point","data":{"latitude":1,"longitude":2}}}`)...)
	issues := v.ValidateObservationHistoryEvent(payload)
	if !hasIssueCode(issues, "limit_exceeded") {
		t.Fatalf("expected limit_exceeded for oversized raw payload, got %#v", issues)
	}
}

type invalidHistoryManifest struct {
	Cases []invalidHistoryCase `json:"cases"`
}

type invalidHistoryCase struct {
	ID       string            `json:"id"`
	Source   string            `json:"source"`
	Expected []ValidationIssue `json:"expected"`
}

func TestObservationHistoryEventInvalidGoldens(t *testing.T) {
	root := protocolRoot(t)
	v, err := NewWithProtocolRoot(root)
	if err != nil {
		t.Fatalf("NewWithProtocolRoot: %v", err)
	}
	var manifest invalidHistoryManifest
	readJSON(t, filepath.Join(root, "source", "manifests", "invalid-history-cases.json"), &manifest)
	for _, tc := range manifest.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			payload, err := os.ReadFile(filepath.Join(root, tc.Source))
			if err != nil {
				t.Fatalf("read payload: %v", err)
			}
			actual := NormalizeValidationIssues(v.ValidateObservationHistoryEvent(payload))
			expected := NormalizeValidationIssues(tc.Expected)
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("issues mismatch\nexpected=%#v\nactual=%#v", expected, actual)
			}
		})
	}
}

func TestObservationHistoryEventSchemaIssuesUseHistoryPrefix(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	payload, err := json.Marshal(map[string]any{
		"event_id":                 "evt-1",
		"event_type":               "identity_patch",
		"recorded_at":              "2026-01-01T00:00:00Z",
		"observation_id":           "obs-1",
		"base_observation_version": 1,
		"payload":                  map[string]any{},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	issues := v.ValidateObservationHistoryEvent(payload)
	if !hasIssue(issues, "history.effective_at", "required") {
		t.Fatalf("expected history.effective_at required, got %#v", issues)
	}
	for _, issue := range issues {
		if issue.Field == "json.effective_at" || issue.Field == "json.observed_at" {
			t.Fatalf("schema issue must use history.* prefix, got %#v", issues)
		}
	}
}

func TestObservationHistoryEventRejectsObservedAtOnNonTelemetry(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, eventType := range []string{"identity_patch", "lifecycle"} {
		payload, err := json.Marshal(map[string]any{
			"event_id":                 "evt-1",
			"event_type":               eventType,
			"recorded_at":              "2026-01-01T00:00:00Z",
			"observation_id":           "obs-1",
			"base_observation_version": 1,
			"payload":                  map[string]any{},
			"observed_at":              nil,
		})
		if err != nil {
			t.Fatalf("marshal %s: %v", eventType, err)
		}
		issues := v.ValidateObservationHistoryEvent(payload)
		if !hasIssue(issues, "history.observed_at", "unknown_field") {
			t.Fatalf("expected unknown_field for history.observed_at on %s, got %#v", eventType, issues)
		}
	}
}

func mustObject(t *testing.T, payload []byte) jsonObject {
	t.Helper()
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	root, ok := value.(map[string]any)
	if !ok {
		t.Fatal("payload must be an object")
	}
	return root
}

func hasIssue(issues []ValidationIssue, field, code string) bool {
	for _, issue := range issues {
		if issue.Field == field && issue.Code == code {
			return true
		}
	}
	return false
}

func hasIssueCode(issues []ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
