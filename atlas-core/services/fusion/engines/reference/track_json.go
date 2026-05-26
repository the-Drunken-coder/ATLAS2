package reference

type trackJSONPayload struct {
	Components            trackComponents `json:"components"`
	CustomReferenceFusion fusionMeta      `json:"custom_reference_fusion"`
}

type trackComponents struct {
	Telemetry     trackTelemetry     `json:"telemetry"`
	FusionSummary trackFusionSummary `json:"fusion_summary"`
}

type trackTelemetry struct {
	ObservedAt         string   `json:"observed_at,omitempty"`
	Latitude           float64  `json:"latitude"`
	Longitude          float64  `json:"longitude"`
	AltitudeM          *float64 `json:"altitude_m,omitempty"`
	UncertaintyRadiusM *float64 `json:"uncertainty_radius_m,omitempty"`
}

type trackFusionSummary struct {
	SourceCount        int    `json:"source_count"`
	Confidence         int    `json:"confidence"`
	ProvenanceObjectID string `json:"provenance_object_id"`
	ObservedAt         string `json:"observed_at,omitempty"`
}

type fusionMeta struct {
	ObservationID string `json:"observation_id"`
}

type provenancePayload struct {
	Kind string `json:"kind"`
}
