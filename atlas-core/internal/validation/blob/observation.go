package blobvalidation

// Top-level keys; custom_* is also allowed via allowedTopLevelKey / isCustomKey (same pattern as entity and task JSON).
var observationAllowedTopLevel = map[string]struct{}{"state": {}, "latest_sighting": {}, "sightings_object_id": {}, "extra": {}}

func validateObservation(root map[string]any, op Operation, violations *[]Violation) {
	_ = op // operation context reserved for future patch-style writes
	validateAllowedTopLevelKeys(root, observationAllowedTopLevel, violations)
	validateExtra(root, violations)
	validateTopLevelCustomSections(root, violations)
	requireObservationState(root, violations)
	optionalString(root, "sightings_object_id", "json.sightings_object_id", violations)
	if latest := optionalObject(root, "latest_sighting", "json.latest_sighting", violations); latest != nil {
		validateOnlyAllowedKeys(latest, "json.latest_sighting", []string{"observed_at", "kind", "data", "extra"}, violations)
		requireRFC3339(latest, "observed_at", "json.latest_sighting.observed_at", violations)
		requireString(latest, "kind", "json.latest_sighting.kind", violations)
		requireObjectField(latest, "data", "json.latest_sighting.data", violations)
		optionalObject(latest, "extra", "json.latest_sighting.extra", violations)
	}
}

func requireObservationState(root map[string]any, violations *[]Violation) {
	state := requireString(root, "state", "json.state", violations)
	if state == "" {
		return
	}
	switch state {
	case "active", "inactive", "ended":
	default:
		appendViolation(violations, "json.state", "INVALID_VALUE", "must be active, inactive, or ended")
	}
}
