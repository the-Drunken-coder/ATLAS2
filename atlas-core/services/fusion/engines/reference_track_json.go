package engines

type referenceTrackJSON struct {
	Components              referenceTrackComponents `json:"components"`
	CustomReferenceFusion   referenceFusionMeta      `json:"custom_reference_fusion"`
}

type referenceTrackComponents struct {
	Telemetry     referenceTrackTelemetry     `json:"telemetry"`
	FusionSummary referenceTrackFusionSummary `json:"fusion_summary"`
}

type referenceTrackTelemetry struct {
	ObservedAt         string   `json:"observed_at,omitempty"`
	Latitude           float64  `json:"latitude"`
	Longitude          float64  `json:"longitude"`
	AltitudeM          *float64 `json:"altitude_m,omitempty"`
	UncertaintyRadiusM *float64 `json:"uncertainty_radius_m,omitempty"`
}

type referenceTrackFusionSummary struct {
	SourceCount        int    `json:"source_count"`
	Confidence         int    `json:"confidence"`
	ProvenanceObjectID string `json:"provenance_object_id"`
	ObservedAt         string `json:"observed_at,omitempty"`
}

type referenceFusionMeta struct {
	ObservationID string `json:"observation_id"`
}

type referenceProvenanceJSON struct {
	Kind string `json:"kind"`
}
