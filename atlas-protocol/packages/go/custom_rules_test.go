package protocol

import (
	"encoding/json"
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
