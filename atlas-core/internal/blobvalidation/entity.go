package blobvalidation

import "github.com/anomalyco/atlas-core/internal/model"

var entityAllowedTopLevel = map[string]struct{}{"components": {}, "extra": {}}

func validateEntity(root map[string]any, entityType model.EntityType, op Operation, violations *[]Violation) {
	validateAllowedTopLevelKeys(root, entityAllowedTopLevel, violations)
	validateExtra(root, violations)
	validateTopLevelCustomSections(root, violations)

	components := requireObjectField(root, "components", "json.components", violations)
	if components == nil {
		return
	}

	allowed := map[string]func(any, string, *[]Violation){}
	required := []string{}
	switch entityType {
	case model.EntityTypeAsset:
		_ = op // operation context reserved for future patch-style writes
		allowed = map[string]func(any, string, *[]Violation){
			"supported_commands": validateSupportedCommands,
			"telemetry":          func(v any, p string, out *[]Violation) { validateTelemetry(v, p, false, out) },
			"status":             validateStatus,
			"heartbeat":          validateHeartbeat,
			"health":             validateHealth,
			"communications":     validateCommunications,
			"sensor_refs":        validateSensorRefs,
		}
		required = []string{"supported_commands"}
	case model.EntityTypeTrack:
		_ = op // operation context reserved for future patch-style writes
		allowed = map[string]func(any, string, *[]Violation){
			"telemetry":      func(v any, p string, out *[]Violation) { validateTelemetry(v, p, true, out) },
			"status":         validateStatus,
			"fusion_summary": validateFusionSummary,
		}
		required = []string{"telemetry"}
	case model.EntityTypeGeofeature:
		_ = op // operation context reserved for future patch-style writes
		allowed = map[string]func(any, string, *[]Violation){
			"geometry": validateGeometry,
			"status":   validateStatus,
		}
		required = []string{"geometry"}
	}

	for _, key := range required {
		if _, ok := components[key]; !ok {
			appendViolation(violations, joinPath("json.components", key), "REQUIRED", "is required")
		}
	}
	for key, value := range components {
		path := joinPath("json.components", key)
		if isCustomKey(key) {
			validateCustomSection(path, value, violations)
			continue
		}
		validator, ok := allowed[key]
		if !ok {
			appendViolation(violations, path, "UNKNOWN_FIELD", "is not allowed for this entity type")
			continue
		}
		validator(value, path, violations)
	}
}
