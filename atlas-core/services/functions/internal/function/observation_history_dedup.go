package function

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
)

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

// appendHistoryEvent appends one validated line to history.ndjson.
// history.ndjson is authoritative; event_ids.ndjson and extra.event_id_index are
// best-effort accelerators. Index or sidecar maintenance failures are logged but
// do not fail the append—dedup remains correct via sidecar scan and event_id hash.
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
	if err := f.appendEventIDSidecar(ctx, historyObjectID, eventID); err != nil {
		f.log.WarnContext(ctx, "observation_history", "history event appended but event_ids.ndjson update failed",
			logging.String("history_object_id", historyObjectID),
			logging.String("event_id", eventID),
			logging.ErrorField(err),
		)
		if markErr := f.markHistoryEventIDSeen(ctx, historyObjectID, eventID); markErr != nil {
			f.log.WarnContext(ctx, "observation_history", "failed to record appended event_id after sidecar update failure",
				logging.String("history_object_id", historyObjectID),
				logging.String("event_id", eventID),
				logging.ErrorField(markErr),
			)
		}
	}
	if err := f.bootstrapHistoryEventIDIndex(ctx, historyObjectID); err != nil {
		f.log.WarnContext(ctx, "observation_history", "history event appended but event_id_index bootstrap failed",
			logging.String("history_object_id", historyObjectID),
			logging.ErrorField(err),
		)
	}
	if err := f.recordHistoryEventID(ctx, historyObjectID, eventID); err != nil {
		f.log.WarnContext(ctx, "observation_history", "history event appended but event_id_index update failed",
			logging.String("history_object_id", historyObjectID),
			logging.String("event_id", eventID),
			logging.ErrorField(err),
		)
		if markErr := f.markHistoryEventIDSeen(ctx, historyObjectID, eventID); markErr != nil {
			f.log.WarnContext(ctx, "observation_history", "failed to record appended event_id after index update failure",
				logging.String("history_object_id", historyObjectID),
				logging.String("event_id", eventID),
				logging.ErrorField(markErr),
			)
		}
	}
	return nil
}

func (f ObservationFunctions) markHistoryEventIDSeen(ctx context.Context, historyObjectID, eventID string) error {
	obj, err := f.objectGateway.GetObject(ctx, historyObjectID)
	if err != nil {
		return err
	}
	state := parseHistoryEventIDIndex(obj.JSON)
	state.ids[eventID] = struct{}{}
	state.complete = false
	return f.persistHistoryEventIDIndex(ctx, historyObjectID, state)
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

func (f ObservationFunctions) readObjectFile(ctx context.Context, historyObjectID, filename string) ([]byte, error) {
	manifest, err := f.objectGateway.GetObjectManifest(ctx, historyObjectID)
	if err == nil && manifest != nil {
		if info, ok := manifest.Files[filename]; ok && info.Size == 0 {
			return nil, nil
		}
	}
	data, err := f.objectGateway.ReadFile(ctx, historyObjectID, filename)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func (f ObservationFunctions) readHistoryNDJSON(ctx context.Context, historyObjectID string) ([]byte, error) {
	return f.readObjectFile(ctx, historyObjectID, ObservationHistoryFilename)
}

func (f ObservationFunctions) appendEventIDSidecar(ctx context.Context, historyObjectID, eventID string) error {
	line := append([]byte(eventID), '\n')
	_, err := f.objectGateway.AppendFile(ctx, historyObjectID, ObservationHistoryEventIDsFilename, line)
	return err
}

func collectPlainEventIDs(data []byte) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, line := range bytes.Split(data, []byte("\n")) {
		id := strings.TrimSpace(string(line))
		if id == "" {
			continue
		}
		ids[id] = struct{}{}
	}
	return ids
}

func eventIDsSidecarBytes(ids map[string]struct{}) []byte {
	if len(ids) == 0 {
		return nil
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	var buf bytes.Buffer
	for i, id := range sorted {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(id)
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (f ObservationFunctions) writeEventIDsSidecar(ctx context.Context, historyObjectID string, ids map[string]struct{}) error {
	payload := eventIDsSidecarBytes(ids)
	if len(payload) == 0 {
		return nil
	}
	_, err := f.objectGateway.WriteFile(ctx, historyObjectID, ObservationHistoryEventIDsFilename, payload)
	return err
}

func (f ObservationFunctions) rebuildLegacyHistoryIndexesFromHistory(ctx context.Context, historyObjectID string, historyData []byte) (historyEventIDIndexState, error) {
	if len(bytes.TrimSpace(historyData)) == 0 {
		return historyEventIDIndexState{ids: map[string]struct{}{}}, nil
	}
	ids := collectHistoryEventIDs(historyData)
	state := historyEventIDIndexState{ids: ids, complete: true}
	if err := f.writeEventIDsSidecar(ctx, historyObjectID, ids); err != nil {
		state.complete = false
	}
	pruneHistoryEventIDIndexState(&state)
	if err := f.persistHistoryEventIDIndex(ctx, historyObjectID, state); err != nil {
		return state, err
	}
	return state, nil
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

// historyContainsEventID is read-only: it never updates the history object index.
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

	sidecar, err := f.readObjectFile(ctx, historyObjectID, ObservationHistoryEventIDsFilename)
	if err != nil {
		return false, err
	}
	if len(bytes.TrimSpace(sidecar)) > 0 {
		ids := collectPlainEventIDs(sidecar)
		for id := range index.ids {
			ids[id] = struct{}{}
		}
		_, found := ids[eventID]
		return found, nil
	}

	historyData, err := f.readHistoryNDJSON(ctx, historyObjectID)
	if err != nil {
		return false, err
	}
	if len(bytes.TrimSpace(historyData)) == 0 {
		return false, nil
	}
	ids := collectHistoryEventIDs(historyData)
	_, found := ids[eventID]
	return found, nil
}

// bootstrapHistoryEventIDIndex rebuilds and persists the event_id index from sidecar or history.ndjson.
func (f ObservationFunctions) bootstrapHistoryEventIDIndex(ctx context.Context, historyObjectID string) error {
	obj, err := f.objectGateway.GetObject(ctx, historyObjectID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}
	index := parseHistoryEventIDIndex(obj.JSON)
	if index.complete {
		return nil
	}

	sidecar, err := f.readObjectFile(ctx, historyObjectID, ObservationHistoryEventIDsFilename)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(sidecar)) > 0 {
		ids := collectPlainEventIDs(sidecar)
		for id := range index.ids {
			ids[id] = struct{}{}
		}
		index.ids = ids
		index.complete = true
		pruneHistoryEventIDIndexState(&index)
		return f.persistHistoryEventIDIndex(ctx, historyObjectID, index)
	}

	historyData, err := f.readHistoryNDJSON(ctx, historyObjectID)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(historyData)) == 0 {
		return nil
	}
	_, err = f.rebuildLegacyHistoryIndexesFromHistory(ctx, historyObjectID, historyData)
	return err
}
