package function

import (
	"bytes"
	"encoding/json"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

func observationJSONHasKey(jsonBytes []byte, key string) bool {
	_, ok, _ := observationJSONRawKey(jsonBytes, key)
	return ok
}

func rejectClientHistoryObjectID(jsonBytes []byte) error {
	if observationJSONHasKey(jsonBytes, "history_object_id") {
		return model.NewFieldError("INVALID_INPUT", "history_object_id is set by the server through ingest", "json.history_object_id")
	}
	return nil
}

func stripObservationJSONKey(jsonBytes []byte, key string) ([]byte, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", "observation json must be a JSON object", "json")
	}
	delete(root, key)
	return json.Marshal(root)
}

// observationJSONForInitialStore returns JSON for the first DB write on update/upsert.
// When identity changed relative to existing, identity is stripped so syncObservationIdentityHistory
// appends history before the row reflects the new identity.
func observationJSONForInitialStore(existing *model.Observation, incoming []byte) ([]byte, error) {
	if existing == nil {
		return incoming, nil
	}
	before, hadBefore, err := parseObservationIdentity(existing.JSON)
	if err != nil {
		return nil, err
	}
	after, hasAfter, err := parseObservationIdentity(incoming)
	if err != nil {
		return nil, err
	}
	if !hasAfter && !hadBefore {
		return incoming, nil
	}
	if hasAfter && hadBefore && !identityChanged(before, after) {
		return incoming, nil
	}
	return stripObservationJSONKey(incoming, "identity")
}

func observationJSONRawKey(jsonBytes []byte, key string) (json.RawMessage, bool, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, false, model.NewFieldError("INVALID_INPUT", "observation json must be a JSON object", "json")
	}
	raw, ok := root[key]
	return raw, ok, nil
}

func applyLatestTelemetryMutationRules(existingJSON, newJSON []byte) ([]byte, error) {
	newVal, newHas := observationJSONRawKeyLenient(newJSON, "latest_telemetry")
	if !newHas {
		existingVal, existingHas := observationJSONRawKeyLenient(existingJSON, "latest_telemetry")
		if !existingHas {
			return newJSON, nil
		}
		merged, err := mergeObservationJSON(newJSON, map[string]any{"latest_telemetry": json.RawMessage(existingVal)})
		if err != nil {
			return nil, err
		}
		return merged, nil
	}
	existingVal, existingHas := observationJSONRawKeyLenient(existingJSON, "latest_telemetry")
	if !existingHas || !bytes.Equal(canonicalJSONBytes(existingVal), canonicalJSONBytes(newVal)) {
		return nil, model.NewFieldError("INVALID_INPUT", "latest_telemetry must be updated through ingest", "json.latest_telemetry")
	}
	return newJSON, nil
}

func observationJSONRawKeyLenient(jsonBytes []byte, key string) (json.RawMessage, bool) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, false
	}
	raw, ok := root[key]
	return raw, ok
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

func parseObservationTelemetry(jsonBytes []byte) (json.RawMessage, bool, error) {
	var root struct {
		LatestTelemetry json.RawMessage `json:"latest_telemetry"`
	}
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, false, model.NewFieldError("INVALID_INPUT", "observation json must be a JSON object", "json")
	}
	if len(root.LatestTelemetry) == 0 {
		return nil, false, nil
	}
	telemetry, err := validateJSONObjectRaw(root.LatestTelemetry, "json.latest_telemetry")
	if err != nil {
		return nil, false, err
	}
	return telemetry, true, nil
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

func observationJSONPatchMap(jsonBytes []byte) (map[string]any, error) {
	root := map[string]any{}
	if len(jsonBytes) == 0 {
		return root, nil
	}
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", "observation json must be a JSON object", "json")
	}
	return root, nil
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
