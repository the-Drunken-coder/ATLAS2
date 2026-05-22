package function

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

func buildTelemetryHistoryLine(observationID string, baseVersion int, telemetry telemetryEnvelope, recordedAt time.Time) ([]byte, error) {
	observedAt, err := telemetry.observedAt()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(telemetryHistoryPayload{
		Kind:  telemetry.Kind,
		Data:  telemetry.Data,
		Extra: telemetry.extraOrEmpty(),
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

func buildIdentityPatchHistoryLine(observationID string, baseVersion int, previous, current json.RawMessage, effectiveAt, recordedAt time.Time) ([]byte, error) {
	payload, err := json.Marshal(identityPatchPayload{
		Previous: previous,
		Current:  current,
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

func applyTelemetryEventToObservation(obs *model.Observation, telemetry telemetryEnvelope, observedAt time.Time) error {
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

func applyIdentityPatchToObservation(obs *model.Observation, identity json.RawMessage, effectiveAt time.Time, historyObjectID string) error {
	root := map[string]any{}
	if len(obs.JSON) > 0 {
		if err := json.Unmarshal(obs.JSON, &root); err != nil {
			return model.NewFieldError("INVALID_INPUT", "observation json must be a JSON object", "json")
		}
	}
	if isJSONNull(identity) {
		delete(root, "identity")
	} else {
		root["identity"] = json.RawMessage(identity)
	}
	root["history_object_id"] = historyObjectID
	merged, err := json.Marshal(root)
	if err != nil {
		return err
	}
	obs.JSON = merged
	utc := effectiveAt.UTC()
	obs.LatestIdentityAt = &utc
	return nil
}

func applyHistoryEventToObservation(obs *model.Observation, evt observationHistoryEvent, historyObjectID string) error {
	switch evt.EventType {
	case observationEventTelemetry:
		var payload telemetryHistoryPayload
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			return err
		}
		telemetry := telemetryEnvelope{
			ObservedAt: evt.ObservedAt,
			Kind:       payload.Kind,
			Data:       payload.Data,
			Extra:      payload.Extra,
		}
		observedAt, err := telemetry.observedAt()
		if err != nil {
			return err
		}
		if telemetryObservedAtIsNewer(obs, observedAt) {
			if err := applyTelemetryEventToObservation(obs, telemetry, observedAt); err != nil {
				return err
			}
		}
		var mergeErr error
		obs.JSON, mergeErr = mergeObservationJSON(obs.JSON, map[string]any{"history_object_id": historyObjectID})
		return mergeErr
	case observationEventIdentityPatch:
		var payload identityPatchPayload
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			return err
		}
		effectiveAt, err := time.Parse(time.RFC3339Nano, evt.EffectiveAt)
		if err != nil {
			return err
		}
		if identityEffectiveAtIsNewer(obs, effectiveAt) {
			return applyIdentityPatchToObservation(obs, payload.Current, effectiveAt, historyObjectID)
		}
		var mergeErr error
		obs.JSON, mergeErr = mergeObservationJSON(obs.JSON, map[string]any{"history_object_id": historyObjectID})
		return mergeErr
	default:
		return model.NewFieldError("INVALID_INPUT", "unsupported history event type: "+evt.EventType, "event_type")
	}
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func (t telemetryEnvelope) observedAt() (time.Time, error) {
	if t.ObservedAt == "" {
		return time.Time{}, model.NewFieldError("INVALID_INPUT", "telemetry observed_at is required", "telemetry.observed_at")
	}
	parsed, err := time.Parse(time.RFC3339Nano, t.ObservedAt)
	if err != nil {
		return time.Time{}, model.NewFieldError("INVALID_INPUT", "telemetry observed_at must be RFC 3339", "telemetry.observed_at")
	}
	return parsed.UTC(), nil
}

func (t telemetryEnvelope) extraOrEmpty() json.RawMessage {
	if len(t.Extra) == 0 {
		return json.RawMessage("{}")
	}
	return t.Extra
}

func (f ObservationFunctions) syncObservationIdentityHistory(ctx context.Context, obs *model.Observation, existing *model.Observation, effectiveAt time.Time) (bool, error) {
	if f.objectGateway == nil {
		return false, nil
	}
	var before json.RawMessage
	hadBefore := false
	if existing != nil {
		var err error
		before, hadBefore, err = parseObservationIdentity(existing.JSON)
		if err != nil {
			return false, err
		}
	}
	after, hasAfter, err := parseObservationIdentity(obs.JSON)
	if err != nil {
		return false, err
	}
	if hasAfter && (!hadBefore || identityChanged(before, after)) {
		var previous json.RawMessage
		if hadBefore {
			previous = before
		}
		if err := f.appendIdentityPatchIfNeeded(ctx, obs, previous, after, effectiveAt); err != nil {
			return false, err
		}
		return true, nil
	}
	if hadBefore && !hasAfter {
		if err := f.appendIdentityPatchIfNeeded(ctx, obs, before, json.RawMessage("null"), effectiveAt); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func observationIdentityEffectiveAt(obs *model.Observation, fallback time.Time) time.Time {
	if obs.LatestIdentityAt != nil {
		return obs.LatestIdentityAt.UTC()
	}
	return fallback.UTC()
}

func telemetryObservedAtIsNewer(obs *model.Observation, observedAt time.Time) bool {
	if obs.LatestTelemetryAt == nil {
		return true
	}
	return observedAt.UTC().After(obs.LatestTelemetryAt.UTC())
}

func identityEffectiveAtIsNewer(obs *model.Observation, effectiveAt time.Time) bool {
	if obs.LatestIdentityAt == nil {
		return true
	}
	return effectiveAt.UTC().After(obs.LatestIdentityAt.UTC())
}

func (f ObservationFunctions) appendTelemetryHistoryIfNeeded(ctx context.Context, historyObjectID string, eventLine []byte) error {
	exists, err := f.historyContainsEventID(ctx, historyObjectID, mustEventID(eventLine))
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return f.appendHistoryEvent(ctx, historyObjectID, eventLine)
}

func (f ObservationFunctions) appendIdentityPatchIfNeeded(ctx context.Context, obs *model.Observation, previous, current json.RawMessage, effectiveAt time.Time) error {
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
	exists, err := f.historyContainsEventID(ctx, historyObjectID, mustEventID(line))
	if err != nil {
		return err
	}
	if !exists {
		if err := f.appendHistoryEvent(ctx, historyObjectID, line); err != nil {
			return err
		}
	}
	return applyIdentityPatchToObservation(obs, current, effectiveAt, historyObjectID)
}
