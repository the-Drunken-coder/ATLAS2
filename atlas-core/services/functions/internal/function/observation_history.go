package function

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
)

const ObservationHistoryFilename = "history.ndjson"

const (
	observationEventTelemetry     = "telemetry"
	observationEventIdentityPatch = "identity_patch"
	observationEventLifecycle     = "lifecycle"
)

type observationHistoryEvent struct {
	EventID                 string          `json:"event_id"`
	EventType               string          `json:"event_type"`
	RecordedAt              string          `json:"recorded_at"`
	ObservedAt              string          `json:"observed_at,omitempty"`
	EffectiveAt             string          `json:"effective_at,omitempty"`
	ObservationID           string          `json:"observation_id"`
	BaseObservationVersion  int             `json:"base_observation_version"`
	Payload                 json.RawMessage `json:"payload"`
}

func generateEventID(observationID, eventType, timestamp string, payload []byte) string {
	canonicalPayload := canonicalJSONBytes(payload)
	h := sha256.New()
	h.Write([]byte(observationID))
	h.Write([]byte{0})
	h.Write([]byte(eventType))
	h.Write([]byte{0})
	h.Write([]byte(timestamp))
	h.Write([]byte{0})
	h.Write(canonicalPayload)
	sum := h.Sum(nil)
	return "obs_evt_" + hex.EncodeToString(sum[:16])
}

func canonicalJSONBytes(payload []byte) []byte {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return payload
	}
	out, err := json.Marshal(value)
	if err != nil {
		return payload
	}
	return out
}

func (f ObservationFunctions) validateHistoryEventLine(line []byte) error {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return model.NewFieldError("INVALID_INPUT", "history event must not be empty", "history")
	}
	if issues := f.protoValidator.ValidateObservationHistoryEvent(trimmed); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	return nil
}

func (f ObservationFunctions) appendHistoryEvent(ctx context.Context, historyObjectID string, line []byte) error {
	if err := f.validateHistoryEventLine(line); err != nil {
		return err
	}
	if _, err := f.objectGateway.AppendFile(ctx, historyObjectID, ObservationHistoryFilename, line); err != nil {
		return err
	}
	return nil
}

func (f ObservationFunctions) historyContainsEventID(ctx context.Context, historyObjectID, eventID string) (bool, error) {
	data, err := f.objectGateway.ReadFile(ctx, historyObjectID, ObservationHistoryFilename)
	if err != nil {
		return false, err
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		var evt observationHistoryEvent
		if err := json.Unmarshal(trimmed, &evt); err != nil {
			continue
		}
		if evt.EventID == eventID {
			return true, nil
		}
	}
	return false, nil
}

func buildTelemetryHistoryLine(observationID string, baseVersion int, telemetry map[string]any, recordedAt time.Time) ([]byte, error) {
	observedAt, err := parseTelemetryObservedAt(telemetry)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"kind": telemetry["kind"],
		"data": telemetry["data"],
		"extra": telemetryExtraOrEmpty(telemetry),
	})
	if err != nil {
		return nil, err
	}
	eventID := generateEventID(observationID, observationEventTelemetry, observedAt.Format(time.RFC3339Nano), payload)
	evt := observationHistoryEvent{
		EventID:                eventID,
		EventType:              observationEventTelemetry,
		RecordedAt:             recordedAt.UTC().Format(time.RFC3339Nano),
		ObservedAt:             observedAt.UTC().Format(time.RFC3339Nano),
		ObservationID:          observationID,
		BaseObservationVersion: baseVersion,
		Payload:                payload,
	}
	line, err := json.Marshal(evt)
	if err != nil {
		return nil, err
	}
	return append(line, '\n'), nil
}

func buildIdentityPatchHistoryLine(observationID string, baseVersion int, previous, current map[string]any, effectiveAt, recordedAt time.Time) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{
		"previous": previous,
		"current":  current,
	})
	if err != nil {
		return nil, err
	}
	effective := effectiveAt.UTC().Format(time.RFC3339Nano)
	eventID := generateEventID(observationID, observationEventIdentityPatch, effective, payload)
	evt := observationHistoryEvent{
		EventID:                eventID,
		EventType:              observationEventIdentityPatch,
		RecordedAt:             recordedAt.UTC().Format(time.RFC3339Nano),
		EffectiveAt:            effective,
		ObservationID:          observationID,
		BaseObservationVersion: baseVersion,
		Payload:                payload,
	}
	line, err := json.Marshal(evt)
	if err != nil {
		return nil, err
	}
	return append(line, '\n'), nil
}

func applyTelemetryEventToObservation(obs *model.Observation, telemetry map[string]any, observedAt time.Time) error {
	telemetryJSON, err := json.Marshal(telemetry)
	if err != nil {
		return err
	}
	root, err := mergeObservationJSON(obs.JSON, map[string]any{
		"latest_telemetry": json.RawMessage(telemetryJSON),
	})
	if err != nil {
		return err
	}
	obs.JSON = root
	utc := observedAt.UTC()
	obs.LatestTelemetryAt = &utc
	return nil
}

func applyIdentityPatchToObservation(obs *model.Observation, identity map[string]any, effectiveAt time.Time, historyObjectID string) error {
	identityJSON, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	patch := map[string]any{
		"identity":            json.RawMessage(identityJSON),
		"history_object_id": historyObjectID,
	}
	root, err := mergeObservationJSON(obs.JSON, patch)
	if err != nil {
		return err
	}
	obs.JSON = root
	utc := effectiveAt.UTC()
	obs.LatestIdentityAt = &utc
	return nil
}

func parseTelemetryObservedAt(telemetry map[string]any) (time.Time, error) {
	raw, ok := telemetry["observed_at"].(string)
	if !ok || raw == "" {
		return time.Time{}, model.NewFieldError("INVALID_INPUT", "telemetry observed_at is required", "telemetry.observed_at")
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, model.NewFieldError("INVALID_INPUT", "telemetry observed_at must be RFC 3339", "telemetry.observed_at")
	}
	return parsed.UTC(), nil
}

func telemetryExtraOrEmpty(telemetry map[string]any) map[string]any {
	if extra, ok := telemetry["extra"].(map[string]any); ok {
		return extra
	}
	return map[string]any{}
}

func observationJSONHasKey(jsonBytes []byte, key string) bool {
	var root map[string]any
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return false
	}
	_, ok := root[key]
	return ok
}

func parseObservationIdentity(jsonBytes []byte) (map[string]any, bool, error) {
	var root map[string]any
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, false, model.NewFieldError("INVALID_INPUT", "observation json must be a JSON object", "json")
	}
	raw, ok := root["identity"]
	if !ok {
		return nil, false, nil
	}
	identity, ok := raw.(map[string]any)
	if !ok {
		return nil, false, model.NewFieldError("INVALID_INPUT", "identity must be an object", "json.identity")
	}
	return identity, true, nil
}

func mergeObservationJSON(jsonBytes []byte, patch map[string]any) ([]byte, error) {
	root := map[string]any{}
	if len(jsonBytes) > 0 {
		if err := json.Unmarshal(jsonBytes, &root); err != nil {
			return nil, model.NewFieldError("INVALID_INPUT", "observation json must be a JSON object", "json")
		}
	}
	for key, value := range patch {
		root[key] = value
	}
	return json.Marshal(root)
}

func identityChanged(before, after map[string]any) bool {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	return !bytes.Equal(b, a)
}

func (f ObservationFunctions) ensureObservationHistoryObject(ctx context.Context, observationID string, now time.Time) (string, error) {
	historyObjectID := ObservationHistoryObjectID(observationID)
	historyObject := &model.Object{
		ObjectID:  historyObjectID,
		Type:      model.ObjectTypeObservationHistory,
		OwnerType: model.OwnerTypeObservation,
		OwnerID:   observationID,
		JSON:      []byte(`{"format_version":"v1"}`),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if issues := f.protoValidator.ValidateObject(historyObject); len(issues) > 0 {
		return "", protocolvalidation.NewValidationError(issues)
	}
	if err := f.objectGateway.EnsureObjectCreated(ctx, historyObject); err != nil {
		return "", err
	}
	if err := validateObservationHistoryObject(historyObject, observationID); err != nil {
		return "", err
	}
	return historyObjectID, nil
}

func (f ObservationFunctions) appendIdentityPatchIfNeeded(ctx context.Context, obs *model.Observation, previous map[string]any, current map[string]any, effectiveAt time.Time) error {
	if f.objectGateway == nil {
		return model.NewFieldError("INTERNAL", "observation object gateway is not configured", "object_gateway")
	}
	now := time.Now().UTC()
	historyObjectID, err := f.ensureObservationHistoryObject(ctx, obs.ObservationID, now)
	if err != nil {
		return err
	}
	line, err := buildIdentityPatchHistoryLine(obs.ObservationID, obs.Version, previous, current, effectiveAt, now)
	if err != nil {
		return err
	}
	if err := f.appendHistoryEvent(ctx, historyObjectID, line); err != nil {
		return err
	}
	return applyIdentityPatchToObservation(obs, current, effectiveAt, historyObjectID)
}

func (f ObservationFunctions) reconcileAfterHistoryAppend(ctx context.Context, obs *model.Observation, historyObjectID string, eventLine []byte) error {
	var evt observationHistoryEvent
	if err := json.Unmarshal(bytes.TrimSpace(eventLine), &evt); err != nil {
		return err
	}
	exists, err := f.historyContainsEventID(ctx, historyObjectID, evt.EventID)
	if err != nil {
		return err
	}
	if !exists {
		if err := f.appendHistoryEvent(ctx, historyObjectID, eventLine); err != nil {
			return err
		}
	}
	switch evt.EventType {
	case observationEventTelemetry:
		var payload map[string]any
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			return err
		}
		payload["observed_at"] = evt.ObservedAt
		observedAt, err := parseTelemetryObservedAt(payload)
		if err != nil {
			return err
		}
		if err := applyTelemetryEventToObservation(obs, payload, observedAt); err != nil {
			return err
		}
	case observationEventIdentityPatch:
		var payload struct {
			Current map[string]any `json:"current"`
		}
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			return err
		}
		effectiveAt, err := time.Parse(time.RFC3339Nano, evt.EffectiveAt)
		if err != nil {
			return err
		}
		if err := applyIdentityPatchToObservation(obs, payload.Current, effectiveAt, historyObjectID); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported event type for reconcile: %s", evt.EventType)
	}
	obs.UpdatedAt = time.Now().UTC()
	return f.pgStore.UpdateObservation(ctx, obs)
}

func canonicalizeTelemetryJSON(telemetryJSON []byte) (map[string]any, error) {
	var telemetry any
	if err := json.Unmarshal(telemetryJSON, &telemetry); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", "telemetry must be valid JSON", "telemetry")
	}
	telemetryObject, ok := telemetry.(map[string]any)
	if !ok {
		return nil, model.NewFieldError("INVALID_INPUT", "telemetry must be a JSON object", "telemetry")
	}
	return telemetryObject, nil
}

func endedAtOrderingValid(startedAt time.Time, endedAt *time.Time) error {
	if endedAt == nil {
		return nil
	}
	if endedAt.Before(startedAt) {
		return model.NewFieldError("INVALID_INPUT", "ended_at must be greater than or equal to started_at", "ended_at")
	}
	return nil
}

func parseIdentityBytes(identityJSON []byte) (map[string]any, error) {
	if len(identityJSON) == 0 {
		return nil, nil
	}
	var identity map[string]any
	if err := json.Unmarshal(identityJSON, &identity); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", "identity must be valid JSON", "identity")
	}
	if identity == nil {
		return nil, model.NewFieldError("INVALID_INPUT", "identity must be a JSON object", "identity")
	}
	return identity, nil
}

func historyEventIDFromLine(line []byte) (string, error) {
	var evt observationHistoryEvent
	if err := json.Unmarshal(bytes.TrimSpace(line), &evt); err != nil {
		return "", err
	}
	if strings.TrimSpace(evt.EventID) == "" {
		return "", fmt.Errorf("history event missing event_id")
	}
	return evt.EventID, nil
}
