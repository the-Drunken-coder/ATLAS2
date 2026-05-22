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

// ObservationHistoryEventIDsFilename is an append-only sidecar (one event_id per line)
// used for dedup without scanning full history.ndjson on the ingest hot path.
const ObservationHistoryEventIDsFilename = "event_ids.ndjson"

// maxHistoryEventIDIndexEntries caps extra.event_id_index size. When exceeded the
// index is truncated and marked incomplete so lookups fall back to event_ids.ndjson.
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
