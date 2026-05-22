package function

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
)

const ObservationHistoryFilename = "history.ndjson"

// maxHistoryEventIDIndexEntries caps extra.event_id_index size. When exceeded the
// index is truncated and marked incomplete so lookups fall back to history.ndjson.
const maxHistoryEventIDIndexEntries = 8192

const (
	observationEventTelemetry     = "telemetry"
	observationEventIdentityPatch = "identity_patch"
	observationEventLifecycle     = "lifecycle"
)

type observationHistoryEvent struct {
	EventID                string          `json:"event_id"`
	EventType              string          `json:"event_type"`
	RecordedAt             string          `json:"recorded_at"`
	ObservedAt             string          `json:"observed_at,omitempty"`
	EffectiveAt            string          `json:"effective_at,omitempty"`
	ObservationID          string          `json:"observation_id"`
	BaseObservationVersion int             `json:"base_observation_version"`
	Payload                json.RawMessage `json:"payload"`
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
	eventID, err := historyEventIDFromLine(line)
	if err != nil {
		return err
	}
	if _, err := f.objectGateway.AppendFile(ctx, historyObjectID, ObservationHistoryFilename, line); err != nil {
		return err
	}
	return f.recordHistoryEventID(ctx, historyObjectID, eventID)
}

type historyEventIDIndexState struct {
	ids      map[string]struct{}
	complete bool
}

func parseHistoryEventIDIndex(jsonBytes []byte) historyEventIDIndexState {
	state := historyEventIDIndexState{ids: map[string]struct{}{}}
	var root observationHistoryObjectJSON
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return state
	}
	state.complete = root.Extra.EventIDIndexComplete
	for _, id := range root.Extra.EventIDIndex {
		if id == "" {
			continue
		}
		state.ids[id] = struct{}{}
	}
	return state
}

func mergeHistoryEventIDIndexIntoJSON(jsonBytes []byte, state historyEventIDIndexState) ([]byte, error) {
	root := map[string]any{}
	if len(jsonBytes) > 0 {
		if err := json.Unmarshal(jsonBytes, &root); err != nil {
			return nil, err
		}
	}
	if _, ok := root["format_version"]; !ok {
		root["format_version"] = "v1"
	}
	pruneHistoryEventIDIndexState(&state)
	ids := make([]string, 0, len(state.ids))
	for id := range state.ids {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	extra, _ := root["extra"].(map[string]any)
	if extra == nil {
		extra = map[string]any{}
	}
	extra["event_id_index"] = ids
	extra["event_id_index_complete"] = state.complete
	root["extra"] = extra
	return json.Marshal(root)
}

func pruneHistoryEventIDIndexState(state *historyEventIDIndexState) {
	if len(state.ids) <= maxHistoryEventIDIndexEntries {
		return
	}
	ids := make([]string, 0, len(state.ids))
	for id := range state.ids {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	keep := ids[len(ids)-maxHistoryEventIDIndexEntries:]
	next := make(map[string]struct{}, len(keep))
	for _, id := range keep {
		next[id] = struct{}{}
	}
	state.ids = next
	state.complete = false
}

func (f ObservationFunctions) persistHistoryEventIDIndex(ctx context.Context, historyObjectID string, state historyEventIDIndexState) error {
	obj, err := f.objectGateway.GetObject(ctx, historyObjectID)
	if err != nil {
		return err
	}
	obj.JSON, err = mergeHistoryEventIDIndexIntoJSON(obj.JSON, state)
	if err != nil {
		return err
	}
	if issues := f.protoValidator.ValidateObject(obj); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	return f.objectGateway.UpdateObject(ctx, obj)
}

func (f ObservationFunctions) recordHistoryEventID(ctx context.Context, historyObjectID, eventID string) error {
	obj, err := f.objectGateway.GetObject(ctx, historyObjectID)
	if err != nil {
		return err
	}
	state := parseHistoryEventIDIndex(obj.JSON)
	state.ids[eventID] = struct{}{}
	return f.persistHistoryEventIDIndex(ctx, historyObjectID, state)
}

func (f ObservationFunctions) readHistoryNDJSON(ctx context.Context, historyObjectID string) ([]byte, error) {
	manifest, err := f.objectGateway.GetObjectManifest(ctx, historyObjectID)
	if err == nil && manifest != nil {
		if info, ok := manifest.Files[ObservationHistoryFilename]; ok && info.Size == 0 {
			return nil, nil
		}
	}
	data, err := f.objectGateway.ReadFile(ctx, historyObjectID, ObservationHistoryFilename)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func collectHistoryEventIDs(data []byte) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, line := range bytes.Split(data, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		id, err := historyEventIDFromLine(trimmed)
		if err != nil {
			continue
		}
		ids[id] = struct{}{}
	}
	return ids
}

func (f ObservationFunctions) historyContainsEventID(ctx context.Context, historyObjectID, eventID string) (bool, error) {
	obj, err := f.objectGateway.GetObject(ctx, historyObjectID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	index := parseHistoryEventIDIndex(obj.JSON)
	if index.complete {
		_, ok := index.ids[eventID]
		return ok, nil
	}
	if _, ok := index.ids[eventID]; ok {
		return true, nil
	}

	data, err := f.readHistoryNDJSON(ctx, historyObjectID)
	if err != nil {
		return false, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return false, nil
	}
	ids := collectHistoryEventIDs(data)
	_, found := ids[eventID]
	index.ids = ids
	index.complete = true
	if err := f.persistHistoryEventIDIndex(ctx, historyObjectID, index); err != nil {
		return false, err
	}
	return found, nil
}

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
	patch := map[string]any{
		"identity":          identity,
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

func observationJSONHasKey(jsonBytes []byte, key string) bool {
	var root map[string]any
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return false
	}
	_, ok := root[key]
	return ok
}

func parseObservationIdentity(jsonBytes []byte) (json.RawMessage, bool, error) {
	var root struct {
		Identity json.RawMessage `json:"identity"`
	}
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, false, model.NewFieldError("INVALID_INPUT", "observation json must be a JSON object", "json")
	}
	if len(root.Identity) == 0 {
		return nil, false, nil
	}
	identity, err := validateJSONObjectRaw(root.Identity, "json.identity")
	if err != nil {
		return nil, false, err
	}
	return identity, true, nil
}

func validateJSONObjectRaw(raw []byte, field string) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, model.NewFieldError("INVALID_INPUT", field+" must not be empty", field)
	}
	if !json.Valid(trimmed) {
		return nil, model.NewFieldError("INVALID_INPUT", field+" must be valid JSON", field)
	}
	if trimmed[0] != '{' {
		return nil, model.NewFieldError("INVALID_INPUT", field+" must be a JSON object", field)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", field+" must be valid JSON", field)
	}
	return json.RawMessage(trimmed), nil
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

func identityChanged(before, after json.RawMessage) bool {
	return !bytes.Equal(canonicalJSONBytes(before), canonicalJSONBytes(after))
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
	// Reload the latest observation from the DB before applying the event
	reloaded, err := f.pgStore.GetObservation(ctx, obs.ObservationID)
	if err != nil {
		return err
	}
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
		if err := applyTelemetryEventToObservation(reloaded, telemetry, observedAt); err != nil {
			return err
		}
		reloaded.JSON, err = mergeObservationJSON(reloaded.JSON, map[string]any{"history_object_id": historyObjectID})
		if err != nil {
			return err
		}
	case observationEventIdentityPatch:
		var payload identityPatchPayload
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			return err
		}
		effectiveAt, err := time.Parse(time.RFC3339Nano, evt.EffectiveAt)
		if err != nil {
			return err
		}
		if err := applyIdentityPatchToObservation(reloaded, payload.Current, effectiveAt, historyObjectID); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported event type for reconcile: %s", evt.EventType)
	}
	reloaded.UpdatedAt = time.Now().UTC()
	return f.pgStore.UpdateObservation(ctx, reloaded)
}

func canonicalizeTelemetryJSON(telemetryJSON []byte) (telemetryEnvelope, error) {
	var telemetry telemetryEnvelope
	if err := json.Unmarshal(telemetryJSON, &telemetry); err != nil {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry must be valid JSON", "telemetry")
	}
	if telemetry.Kind == "" {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry kind is required", "telemetry.kind")
	}
	data := bytes.TrimSpace(telemetry.Data)
	if len(data) == 0 || data[0] != '{' {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry data must be an object", "telemetry.data")
	}
	if !json.Valid(data) {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry data must be valid JSON", "telemetry.data")
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry data must be an object", "telemetry.data")
	}
	telemetry.Data = json.RawMessage(data)
	return telemetry, nil
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

func parseIdentityBytes(identityJSON []byte) (json.RawMessage, error) {
	if len(identityJSON) == 0 {
		return nil, nil
	}
	return validateJSONObjectRaw(identityJSON, "identity")
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
