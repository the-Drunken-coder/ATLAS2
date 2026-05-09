package blobvalidation

import "github.com/anomalyco/atlas-core/internal/model"

var entityAllowedTopLevel = map[string]struct{}{"components": {}, "extra": {}}

func validateEntity(root map[string]any, entityType model.EntityType, op Operation, violations *[]Violation) {
	_ = op // operation context reserved for future patch-style writes
	validateAllowedTopLevelKeys(root, entityAllowedTopLevel, violations)
	validateExtra(root, violations)
	validateTopLevelCustomSections(root, violations)

	var allowed map[string]func(any, string, *[]Violation)
	required := []string{}
	entityKindKnown := false
	switch entityType {
	case model.EntityTypeAsset:
		entityKindKnown = true
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
		entityKindKnown = true
		allowed = map[string]func(any, string, *[]Violation){
			"telemetry":      func(v any, p string, out *[]Violation) { validateTelemetry(v, p, true, out) },
			"status":         validateStatus,
			"fusion_summary": validateFusionSummary,
		}
		required = []string{"telemetry"}
	case model.EntityTypeGeofeature:
		entityKindKnown = true
		allowed = map[string]func(any, string, *[]Violation){
			"geometry": validateGeometry,
			"status":   validateStatus,
		}
		required = []string{"geometry"}
	}

	var components map[string]any
	if entityKindKnown && len(required) == 0 {
		components = ensureObjectField(root, "components", violations)
	} else {
		components = requireObjectFieldOrEmpty(root, "components", "json.components", violations)
	}
	if components == nil {
		return
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
