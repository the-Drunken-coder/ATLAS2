package function

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	root, err := observationJSONObjectFromBytes(obs.JSON)
	if err != nil {
		return err
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

type identityHistoryCommit struct {
	historyObjectID string
	eventLine       []byte
	changed         bool
}

// prepareIdentityHistoryCommit builds an identity_patch line and applies identity fields in-memory without appending.
func (f ObservationFunctions) prepareIdentityHistoryCommit(ctx context.Context, obs *model.Observation, existing *model.Observation, effectiveAt time.Time) (identityHistoryCommit, error) {
	if f.objectGateway == nil {
		return identityHistoryCommit{}, nil
	}
	var before json.RawMessage
	hadBefore := false
	if existing != nil {
		var err error
		before, hadBefore, err = parseObservationIdentity(existing.JSON)
		if err != nil {
			return identityHistoryCommit{}, err
		}
	}
	after, hasAfter, err := parseObservationIdentity(obs.JSON)
	if err != nil {
		return identityHistoryCommit{}, err
	}

	var previous json.RawMessage
	var current json.RawMessage
	changed := false
	switch {
	case hasAfter && (!hadBefore || identityChanged(before, after)):
		if hadBefore {
			previous = before
		}
		current = after
		changed = true
	case hadBefore && !hasAfter:
		previous = before
		current = json.RawMessage("null")
		changed = true
	}
	if !changed {
		return identityHistoryCommit{}, nil
	}

	now := time.Now().UTC()
	historyObjectID, err := f.ensureObservationHistoryObject(ctx, obs.ObservationID, now)
	if err != nil {
		return identityHistoryCommit{}, err
	}
	baseVersion := obs.Version
	if existing != nil {
		baseVersion = existing.Version
	}
	line, err := buildIdentityPatchHistoryLine(obs.ObservationID, baseVersion, previous, current, effectiveAt, now)
	if err != nil {
		return identityHistoryCommit{}, err
	}
	if identityEffectiveAtIsNewer(obs, effectiveAt) {
		if err := applyIdentityPatchToObservation(obs, current, effectiveAt, historyObjectID); err != nil {
			return identityHistoryCommit{}, err
		}
	}
	return identityHistoryCommit{
		historyObjectID: historyObjectID,
		eventLine:       line,
		changed:         true,
	}, nil
}

func (f ObservationFunctions) appendPreparedIdentityHistory(ctx context.Context, commit identityHistoryCommit) error {
	if !commit.changed || len(commit.eventLine) == 0 {
		return nil
	}
	return f.appendHistoryEventIfAbsent(ctx, commit.historyObjectID, commit.eventLine)
}

func (f ObservationFunctions) appendIdentityHistoryOrRollbackCreated(ctx context.Context, observationID string, commit identityHistoryCommit) error {
	if err := f.appendPreparedIdentityHistory(ctx, commit); err != nil {
		if rollbackErr := f.pgStore.DeleteObservation(ctx, observationID); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	return nil
}

func (f ObservationFunctions) appendIdentityHistoryOrRollbackUpdated(ctx context.Context, previous *model.Observation, commit identityHistoryCommit) error {
	if err := f.appendPreparedIdentityHistory(ctx, commit); err != nil {
		if previous == nil {
			return err
		}
		rollback := *previous
		if rollbackErr := f.pgStore.UpsertObservation(ctx, &rollback); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	return nil
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
	return f.appendHistoryEventIfAbsent(ctx, historyObjectID, eventLine)
}

func (f ObservationFunctions) appendIdentityPatchIfNeeded(ctx context.Context, obs *model.Observation, previous, current json.RawMessage, effectiveAt time.Time) (identityHistoryCommit, error) {
	if f.objectGateway == nil {
		return identityHistoryCommit{}, model.NewFieldError("INTERNAL", "observation object gateway is not configured", "object_gateway")
	}
	now := time.Now().UTC()
	historyObjectID, err := f.ensureObservationHistoryObject(ctx, obs.ObservationID, now)
	if err != nil {
		return identityHistoryCommit{}, err
	}
	line, err := buildIdentityPatchHistoryLine(obs.ObservationID, obs.Version, previous, current, effectiveAt, now)
	if err != nil {
		return identityHistoryCommit{}, err
	}
	if err := f.appendHistoryEventIfAbsent(ctx, historyObjectID, line); err != nil {
		return identityHistoryCommit{}, err
	}
	if identityEffectiveAtIsNewer(obs, effectiveAt) {
		if err := applyIdentityPatchToObservation(obs, current, effectiveAt, historyObjectID); err != nil {
			return identityHistoryCommit{}, err
		}
	}
	return identityHistoryCommit{
		historyObjectID: historyObjectID,
		eventLine:       line,
		changed:         true,
	}, nil
}
