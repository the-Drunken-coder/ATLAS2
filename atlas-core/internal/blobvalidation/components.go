package blobvalidation

import (
	"fmt"
	"math"
)

func validateSupportedCommands(value any, path string, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	validateOnlyAllowedKeys(obj, path, []string{"commands"}, violations)
	items := optionalArray(obj, "commands", joinPath(path, "commands"), violations)
	if items == nil {
		if _, exists := obj["commands"]; !exists {
			appendViolation(violations, joinPath(path, "commands"), "REQUIRED", "is required")
		}
		return
	}
	seen := map[string]struct{}{}
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			appendViolation(violations, path+".commands", "INVALID_TYPE", "must contain only strings")
			continue
		}
		if text == "" {
			appendViolation(violations, joinPath(path, "commands"), "INVALID_VALUE", "must not contain empty strings")
		}
		if _, exists := seen[text]; exists {
			appendViolation(violations, joinPath(path, "commands"), "INVALID_VALUE", "must not contain duplicate commands")
			continue
		}
		seen[text] = struct{}{}
	}
}

func validateTelemetry(value any, path string, requirePosition bool, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	validateOnlyAllowedKeys(obj, path, []string{"observed_at", "latitude", "longitude", "altitude_m", "speed_m_s", "heading_deg", "uncertainty_radius_m"}, violations)
	optionalRFC3339(obj, "observed_at", joinPath(path, "observed_at"), violations)
	lat, hasLat := optionalNumber(obj, "latitude", joinPath(path, "latitude"), violations)
	lon, hasLon := optionalNumber(obj, "longitude", joinPath(path, "longitude"), violations)
	if hasLat && (lat < -90 || lat > 90) {
		appendViolation(violations, joinPath(path, "latitude"), "OUT_OF_RANGE", "must be between -90 and 90")
	}
	if hasLon && (lon < -180 || lon > 180) {
		appendViolation(violations, joinPath(path, "longitude"), "OUT_OF_RANGE", "must be between -180 and 180")
	}
	if hasLat != hasLon {
		appendViolation(violations, path, "INVALID_VALUE", "latitude and longitude must be provided together")
	}
	if requirePosition && (!hasLat || !hasLon) {
		appendViolation(violations, path, "REQUIRED", "latitude and longitude are required")
	}
	if speed, ok := optionalNumber(obj, "speed_m_s", joinPath(path, "speed_m_s"), violations); ok && speed < 0 {
		appendViolation(violations, joinPath(path, "speed_m_s"), "OUT_OF_RANGE", "must be greater than or equal to 0")
	}
	if heading, ok := optionalNumber(obj, "heading_deg", joinPath(path, "heading_deg"), violations); ok && (heading < 0 || heading >= 360) {
		appendViolation(violations, joinPath(path, "heading_deg"), "OUT_OF_RANGE", "must be greater than or equal to 0 and less than 360")
	}
	if uncertainty, ok := optionalNumber(obj, "uncertainty_radius_m", joinPath(path, "uncertainty_radius_m"), violations); ok {
		if uncertainty < 0 {
			appendViolation(violations, joinPath(path, "uncertainty_radius_m"), "OUT_OF_RANGE", "must be greater than or equal to 0")
		}
		if !hasLat || !hasLon {
			appendViolation(violations, joinPath(path, "uncertainty_radius_m"), "INVALID_VALUE", "requires latitude and longitude")
		}
	}
	_, _ = optionalNumber(obj, "altitude_m", joinPath(path, "altitude_m"), violations)
}

func validateStatus(value any, path string, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	validateOnlyAllowedKeys(obj, path, []string{"state", "label", "priority"}, violations)
	optionalNonEmptyString(obj, "state", joinPath(path, "state"), violations)
	optionalString(obj, "label", joinPath(path, "label"), violations)
	if priority, ok := optionalInteger(obj, "priority", joinPath(path, "priority"), violations); ok && priority < 0 {
		appendViolation(violations, joinPath(path, "priority"), "OUT_OF_RANGE", "must be greater than or equal to 0")
	}
}

func validateGeometry(value any, path string, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	validateOnlyAllowedKeys(obj, path, []string{"type", "coordinates"}, violations)
	requireString(obj, "type", joinPath(path, "type"), violations)
	coords, ok := obj["coordinates"]
	if !ok {
		appendViolation(violations, joinPath(path, "coordinates"), "REQUIRED", "is required")
		return
	}
	if _, ok := coords.([]any); !ok {
		appendViolation(violations, joinPath(path, "coordinates"), "INVALID_TYPE", "must be an array")
	}
}

func validateHeartbeat(value any, path string, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	validateOnlyAllowedKeys(obj, path, []string{"observed_at", "source", "sequence"}, violations)
	optionalRFC3339(obj, "observed_at", joinPath(path, "observed_at"), violations)
	optionalNonEmptyString(obj, "source", joinPath(path, "source"), violations)
	if seq, ok := optionalInteger(obj, "sequence", joinPath(path, "sequence"), violations); ok && seq < 0 {
		appendViolation(violations, joinPath(path, "sequence"), "OUT_OF_RANGE", "must be greater than or equal to 0")
	}
}

func validateHealth(value any, path string, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	validateOnlyAllowedKeys(obj, path, []string{"state", "battery_percent", "faults"}, violations)
	optionalNonEmptyString(obj, "state", joinPath(path, "state"), violations)
	if pct, ok := optionalNumber(obj, "battery_percent", joinPath(path, "battery_percent"), violations); ok && (pct < 0 || pct > 100) {
		appendViolation(violations, joinPath(path, "battery_percent"), "OUT_OF_RANGE", "must be between 0 and 100")
	}
	faults := optionalArray(obj, "faults", joinPath(path, "faults"), violations)
	for _, item := range faults {
		if _, ok := item.(string); !ok {
			appendViolation(violations, joinPath(path, "faults"), "INVALID_TYPE", "must contain only strings")
		}
	}
}

func validateCommunications(value any, path string, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	validateOnlyAllowedKeys(obj, path, []string{"links"}, violations)
	links := optionalArray(obj, "links", joinPath(path, "links"), violations)
	for idx, item := range links {
		link, ok := item.(map[string]any)
		if !ok {
			appendViolation(violations, joinPath(path, "links"), "INVALID_TYPE", "must contain only objects")
			continue
		}
		linkPath := fmt.Sprintf("%s.links[%d]", path, idx)
		validateOnlyAllowedKeys(link, linkPath, []string{"type", "status", "address", "rssi_dbm", "snr_db"}, violations)
		optionalNonEmptyString(link, "type", joinPath(linkPath, "type"), violations)
		optionalNonEmptyString(link, "status", joinPath(linkPath, "status"), violations)
		optionalString(link, "address", joinPath(linkPath, "address"), violations)
		_, _ = optionalNumber(link, "rssi_dbm", joinPath(linkPath, "rssi_dbm"), violations)
		_, _ = optionalNumber(link, "snr_db", joinPath(linkPath, "snr_db"), violations)
	}
}

func validateSensorRefs(value any, path string, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	validateOnlyAllowedKeys(obj, path, []string{"sensors"}, violations)
	sensors := optionalArray(obj, "sensors", joinPath(path, "sensors"), violations)
	for idx, item := range sensors {
		sensor, ok := item.(map[string]any)
		if !ok {
			appendViolation(violations, joinPath(path, "sensors"), "INVALID_TYPE", "must contain only objects")
			continue
		}
		sensorPath := fmt.Sprintf("%s.sensors[%d]", path, idx)
		validateOnlyAllowedKeys(sensor, sensorPath, []string{"sensor_id", "type", "label", "object_id", "mount"}, violations)
		optionalNonEmptyString(sensor, "sensor_id", joinPath(sensorPath, "sensor_id"), violations)
		optionalNonEmptyString(sensor, "type", joinPath(sensorPath, "type"), violations)
		optionalString(sensor, "label", joinPath(sensorPath, "label"), violations)
		optionalString(sensor, "object_id", joinPath(sensorPath, "object_id"), violations)
		mount := optionalObject(sensor, "mount", joinPath(sensorPath, "mount"), violations)
		if mount == nil {
			continue
		}
		mountPath := joinPath(sensorPath, "mount")
		validateOnlyAllowedKeys(mount, mountPath, []string{"location", "bearing_deg", "elevation_deg", "roll_deg"}, violations)
		optionalNonEmptyString(mount, "location", joinPath(mountPath, "location"), violations)
		if bearing, ok := optionalNumber(mount, "bearing_deg", joinPath(mountPath, "bearing_deg"), violations); ok && (bearing < 0 || bearing >= 360) {
			appendViolation(violations, joinPath(mountPath, "bearing_deg"), "OUT_OF_RANGE", "must be greater than or equal to 0 and less than 360")
		}
		if elev, ok := optionalNumber(mount, "elevation_deg", joinPath(mountPath, "elevation_deg"), violations); ok && (elev < -90 || elev > 90) {
			appendViolation(violations, joinPath(mountPath, "elevation_deg"), "OUT_OF_RANGE", "must be between -90 and 90")
		}
		if roll, ok := optionalNumber(mount, "roll_deg", joinPath(mountPath, "roll_deg"), violations); ok && (roll < 0 || roll >= 360) {
			appendViolation(violations, joinPath(mountPath, "roll_deg"), "OUT_OF_RANGE", "must be greater than or equal to 0 and less than 360")
		}
	}
}

func validateFusionSummary(value any, path string, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	validateOnlyAllowedKeys(obj, path, []string{"observed_at", "source_count", "confidence", "provenance_object_id"}, violations)
	optionalRFC3339(obj, "observed_at", joinPath(path, "observed_at"), violations)
	if count, ok := optionalInteger(obj, "source_count", joinPath(path, "source_count"), violations); ok && count < 0 {
		appendViolation(violations, joinPath(path, "source_count"), "OUT_OF_RANGE", "must be greater than or equal to 0")
	}
	if confidence, ok := optionalNumber(obj, "confidence", joinPath(path, "confidence"), violations); ok && (confidence < 0 || confidence > 1) {
		appendViolation(violations, joinPath(path, "confidence"), "OUT_OF_RANGE", "must be between 0 and 1")
	}
	optionalString(obj, "provenance_object_id", joinPath(path, "provenance_object_id"), violations)
}

func isPositiveInteger(value float64) bool {
	return math.Trunc(value) == value && value > 0
}
