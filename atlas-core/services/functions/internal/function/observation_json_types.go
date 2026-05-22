package function

import "encoding/json"

// telemetryEnvelope matches atlas-protocol observation.schema.json telemetryEnvelope.
type telemetryEnvelope struct {
	ObservedAt string          `json:"observed_at"`
	Kind       string          `json:"kind"`
	Data       json.RawMessage `json:"data"`
	Extra      json.RawMessage `json:"extra,omitempty"`
}

// telemetryHistoryPayload is the observation history event payload for telemetry events.
type telemetryHistoryPayload struct {
	Kind  string          `json:"kind"`
	Data  json.RawMessage `json:"data"`
	Extra json.RawMessage `json:"extra,omitempty"`
}

type identityPatchPayload struct {
	Previous json.RawMessage `json:"previous,omitempty"`
	Current  json.RawMessage `json:"current"`
}

type observationHistoryExtra struct {
	EventIDIndex         []string `json:"event_id_index,omitempty"`
	EventIDIndexComplete bool     `json:"event_id_index_complete,omitempty"`
}

type observationHistoryObjectJSON struct {
	FormatVersion string                  `json:"format_version"`
	Extra         observationHistoryExtra `json:"extra,omitempty"`
}
