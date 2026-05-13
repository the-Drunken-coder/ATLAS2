package protocol

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type object = map[string]any
type coord [2]float64

var promotedFields = map[string]struct{}{
	"entity_id": {}, "object_id": {}, "task_id": {}, "observation_id": {},
	"type": {}, "status": {}, "owner_type": {}, "owner_id": {}, "asset_id": {},
	"source_asset_id": {}, "command_catalog_object_id": {}, "created_at": {},
	"updated_at": {}, "version": {},
}

var standardGeoJSONTypes = map[string]struct{}{
	"Point": {}, "MultiPoint": {}, "LineString": {}, "MultiLineString": {},
	"Polygon": {}, "MultiPolygon": {}, "GeometryCollection": {},
}

func compileProtocolSchemas(root string) error {
	schemaDir := filepath.Join(root, "source", "schemas")
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return fmt.Errorf("read schema directory: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(schemaDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read schema %s: %w", entry.Name(), err)
		}
		var doc any
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse schema %s: %w", entry.Name(), err)
		}
		if err := compiler.AddResource(entry.Name(), doc); err != nil {
			return fmt.Errorf("add schema %s: %w", entry.Name(), err)
		}
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		if _, err := compiler.Compile(entry.Name()); err != nil {
			return fmt.Errorf("compile schema %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func (v *Validator) ValidateBytes(kind ResourceKind, payload []byte, opts ...ValidateOption) []ValidationIssue {
	var o validateOptions
	for _, opt := range opts {
		opt(&o)
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return []ValidationIssue{{Field: "json", Code: "invalid_json", Message: "json must be valid JSON"}}
	}
	root, ok := value.(map[string]any)
	if !ok {
		return []ValidationIssue{{Field: "json", Code: "invalid_type", Message: "json must be an object"}}
	}
	issues := collectLimitIssues("json", root, payload, 64*1024, 16, 500, 100)
	switch kind {
	case ResourceEntity:
		issues = append(issues, validateEntity(root, o.variant)...)
	case ResourceTask:
		issues = append(issues, validateTask(root)...)
	case ResourceObservation:
		issues = append(issues, validateObservation(root)...)
	case ResourceObject:
		issues = append(issues, validateObject(root, o.variant)...)
	case ResourceCommandCatalog:
		issues = append(issues, validateCommandCatalog(root)...)
	case ResourceChangeEvent:
		issues = append(issues, validateChangeEvent(root)...)
	case ResourceCustomSection:
		issues = append(issues, validateCustomSection(root)...)
	default:
		issues = append(issues, ValidationIssue{Field: "resource", Code: "invalid_value", Message: fmt.Sprintf("unknown resource kind: %s", kind)})
	}
	return dedupe(issues)
}

func validateEntity(root object, variant string) []ValidationIssue {
	issues := collectTopLevel(root, []string{"components", "extra"}, "entity")
	issues = append(issues, collectCustom(root, "json")...)
	components, hasComponents := asObject(root["components"])
	if root["components"] != nil && !hasComponents {
		return append(issues, ValidationIssue{Field: "json.components", Code: "invalid_type", Message: "components must be an object"})
	}
	if variant == "asset" {
		if !hasComponents || components["supported_commands"] == nil {
			issues = append(issues, ValidationIssue{Field: "json.components.supported_commands", Code: "required", Message: "supported_commands is required for asset entity JSON"})
		}
	}
	if variant == "track" {
		if !hasComponents || components["telemetry"] == nil {
			issues = append(issues, ValidationIssue{Field: "json.components.telemetry", Code: "required", Message: "telemetry is required"})
		}
	}
	if variant == "geofeature" {
		if !hasComponents || components["geometry"] == nil {
			issues = append(issues, ValidationIssue{Field: "json.components.geometry", Code: "required", Message: "geometry is required"})
		}
	}
	if hasComponents {
		for key := range components {
			if !allowedEntityComponent(variant, key) && !strings.HasPrefix(key, "custom_") {
				issues = append(issues, ValidationIssue{Field: "json.components." + key, Code: "unknown_field", Message: key + " is not allowed"})
			}
		}
		if telemetry, ok := asObject(components["telemetry"]); ok {
			_, hasLat := telemetry["latitude"]
			_, hasLon := telemetry["longitude"]
			if hasLat != hasLon {
				missing := "latitude"
				if hasLat {
					missing = "longitude"
				}
				issues = append(issues, ValidationIssue{Field: "json.components.telemetry." + missing, Code: "required_pair", Message: "latitude and longitude must be provided together"})
			}
			checkRange(&issues, telemetry, "observed_at", "json.components.telemetry.observed_at", 0, 0, false, "date-time")
		}
		if geometry, ok := asObject(components["geometry"]); ok && variant == "geofeature" {
			issues = append(issues, validateGeometry(geometry, "json.components.geometry", standardGeoJSONTypes)...)
		}
	}
	if variant == "" {
		issues = append(issues, ValidationIssue{Field: "json", Code: "invalid_value", Message: "entity variant is required"})
	}
	return issues
}

func validateTask(root object) []ValidationIssue {
	issues := collectTopLevel(root, []string{"description", "created_by", "components", "extra"}, "task")
	issues = append(issues, collectCustom(root, "json")...)
	components, ok := asObject(root["components"])
	if root["components"] != nil && !ok {
		return append(issues, ValidationIssue{Field: "json.components", Code: "invalid_type", Message: "components must be an object"})
	}
	if !ok {
		issues = append(issues, ValidationIssue{Field: "json.components.command.type", Code: "required", Message: "command.type is required"})
		issues = append(issues, ValidationIssue{Field: "json.components", Code: "required", Message: "components is required"})
		return issues
	}
	command, commandOK := asObject(components["command"])
	if !commandOK {
		issues = append(issues, ValidationIssue{Field: "json.components.command.type", Code: "required", Message: "command.type is required"})
		issues = append(issues, ValidationIssue{Field: "json.components.command", Code: "required", Message: "command is required"})
	} else if s, _ := command["type"].(string); s == "" {
		issues = append(issues, ValidationIssue{Field: "json.components.command.type", Code: "required", Message: "command.type is required"})
	}
	if components["parameters"] == nil {
		issues = append(issues, ValidationIssue{Field: "json.components.parameters", Code: "required", Message: "parameters is required"})
	} else if _, ok := asObject(components["parameters"]); !ok {
		issues = append(issues, ValidationIssue{Field: "json.components.parameters", Code: "invalid_type", Message: "parameters must be an object"})
	}
	for _, key := range []string{"progress", "result", "error"} {
		if components[key] != nil {
			if _, ok := asObject(components[key]); !ok {
				issues = append(issues, ValidationIssue{Field: "json.components." + key, Code: "invalid_type", Message: key + " must be an object"})
			}
		}
	}
	for key := range components {
		if !in(key, []string{"command", "parameters", "progress", "result", "error"}) && !strings.HasPrefix(key, "custom_") {
			issues = append(issues, ValidationIssue{Field: "json.components." + key, Code: "unknown_field", Message: key + " is not allowed"})
		}
	}
	return issues
}

func validateObservation(root object) []ValidationIssue {
	issues := collectTopLevel(root, []string{"state", "latest_sighting", "sightings_object_id", "extra"}, "observation")
	issues = append(issues, collectCustom(root, "json")...)
	state, hasState := root["state"].(string)
	if !hasState {
		if root["state"] == nil {
			issues = append(issues, ValidationIssue{Field: "json.state", Code: "required", Message: "state is required"})
		} else {
			issues = append(issues, ValidationIssue{Field: "json.state", Code: "invalid_type", Message: "state must be a string"})
		}
	} else if !in(state, []string{"active", "inactive", "ended"}) {
		issues = append(issues, ValidationIssue{Field: "json.state", Code: "invalid_value", Message: "state must be one of active, inactive, ended"})
	}
	if root["latest_sighting"] != nil {
		sighting, ok := asObject(root["latest_sighting"])
		if !ok {
			issues = append(issues, ValidationIssue{Field: "json.latest_sighting", Code: "invalid_type", Message: "latest_sighting must be an object"})
			return issues
		}
		if sighting["observed_at"] == nil {
			issues = append(issues, ValidationIssue{Field: "json.latest_sighting.observed_at", Code: "required", Message: "observed_at is required"})
		} else {
			checkRange(&issues, sighting, "observed_at", "json.latest_sighting.observed_at", 0, 0, false, "date-time")
		}
		if sighting["data"] == nil {
			issues = append(issues, ValidationIssue{Field: "json.latest_sighting.data", Code: "required", Message: "data is required"})
		} else if _, ok := asObject(sighting["data"]); !ok {
			issues = append(issues, ValidationIssue{Field: "json.latest_sighting.data", Code: "invalid_type", Message: "data must be an object"})
		}
		if sighting["extra"] != nil {
			if _, ok := asObject(sighting["extra"]); !ok {
				issues = append(issues, ValidationIssue{Field: "json.latest_sighting.extra", Code: "invalid_type", Message: "extra must be an object"})
			}
		}
		issues = append(issues, validateSighting(sighting, "json.latest_sighting")...)
	}
	return issues
}

func validateObject(root object, variant string) []ValidationIssue {
	allowed := map[string][]string{
		"log":      {"log_type", "started_at", "ended_at", "extra"},
		"photo":    {"content_type", "captured_at", "width_px", "height_px", "extra"},
		"document": {"content_type", "extra"},
	}
	issues := collectTopLevel(root, allowed[variant], "object")
	issues = append(issues, collectCustom(root, "json")...)
	for _, key := range []string{"manifest", "manifest_version"} {
		if root[key] != nil {
			issues = append(issues, ValidationIssue{Field: "json." + key, Code: "reserved_field", Message: key + " is reserved for internal manifest cache writes"})
		}
	}
	if variant == "" {
		issues = append(issues, ValidationIssue{Field: "json", Code: "invalid_value", Message: "object variant is required"})
	}
	return issues
}

func validateCommandCatalog(root object) []ValidationIssue {
	issues := collectTopLevel(root, []string{"type", "name", "description", "commands"}, "commandCatalog")
	required := []string{"type", "name", "description", "commands"}
	for _, key := range required {
		if root[key] == nil {
			issues = append(issues, ValidationIssue{Field: "json." + key, Code: "required", Message: key + " is required"})
		}
	}
	if root["type"] != nil && root["type"] != "command_catalog" {
		issues = append(issues, ValidationIssue{Field: "json.type", Code: "invalid_value", Message: "type must be one of command_catalog"})
	}
	commands, ok := root["commands"].([]any)
	if root["commands"] != nil && !ok {
		issues = append(issues, ValidationIssue{Field: "json.commands", Code: "invalid_type", Message: "commands must be an array"})
	}
	seen := map[string]struct{}{}
	for i, raw := range commands {
		cmd, ok := asObject(raw)
		if !ok {
			issues = append(issues, ValidationIssue{Field: fmt.Sprintf("json.commands[%d]", i), Code: "invalid_type", Message: fmt.Sprintf("commands[%d] must be an object", i)})
			continue
		}
		for key := range cmd {
			if !in(key, []string{"id", "name", "description", "parameters_schema"}) {
				msg := key + " is not allowed"
				if key == "parameter_schema" {
					msg = "\"parameter_schema\" is not allowed; use \"parameters_schema\""
				}
				issues = append(issues, ValidationIssue{Field: fmt.Sprintf("json.commands[%d].%s", i, key), Code: "unknown_field", Message: msg})
			}
		}
		for _, key := range []string{"id", "name", "description", "parameters_schema"} {
			if cmd[key] == nil {
				issues = append(issues, ValidationIssue{Field: fmt.Sprintf("json.commands[%d].%s", i, key), Code: "required", Message: key + " is required"})
			}
		}
		if id, ok := cmd["id"].(string); ok {
			if _, exists := seen[id]; exists {
				issues = append(issues, ValidationIssue{Field: fmt.Sprintf("json.commands[%d].id", i), Code: "duplicate_command_id", Message: fmt.Sprintf("command id %q must be unique", id)})
			}
			seen[id] = struct{}{}
		}
		params, ok := asObject(cmd["parameters_schema"])
		if cmd["parameters_schema"] != nil && !ok {
			issues = append(issues, ValidationIssue{Field: fmt.Sprintf("json.commands[%d].parameters_schema", i), Code: "invalid_type", Message: "parameters_schema must be an object"})
		}
		for name, rawDef := range params {
			def, ok := asObject(rawDef)
			field := fmt.Sprintf("json.commands[%d].parameters_schema.%s", i, name)
			if !ok {
				issues = append(issues, ValidationIssue{Field: field, Code: "invalid_type", Message: name + " must be an object"})
				continue
			}
			for key := range def {
				if !in(key, []string{"type", "description", "required"}) {
					issues = append(issues, ValidationIssue{Field: field + "." + key, Code: "unknown_field", Message: key + " is not allowed"})
				}
			}
			t, ok := def["type"].(string)
			if !ok {
				issues = append(issues, ValidationIssue{Field: field + ".type", Code: "required", Message: "type is required"})
			} else if !in(t, []string{"string", "number", "boolean", "object", "array"}) {
				issues = append(issues, ValidationIssue{Field: field + ".type", Code: "invalid_value", Message: "type must be one of string, number, boolean, object, array"})
			}
		}
	}
	return issues
}

func validateChangeEvent(root object) []ValidationIssue {
	var issues []ValidationIssue
	allowedFields := []string{"type", "event_id", "resource", "operation", "resource_id", "resource_version", "occurred_at", "snapshot", "metadata"}
	for key := range root {
		if !in(key, allowedFields) {
			issues = append(issues, ValidationIssue{Field: "json." + key, Code: "unknown_field", Message: key + " is not allowed"})
		}
	}
	for _, key := range []string{"type", "event_id", "resource", "operation", "resource_id", "resource_version", "occurred_at", "snapshot"} {
		if _, ok := root[key]; !ok {
			issues = append(issues, ValidationIssue{Field: "json." + key, Code: "required", Message: key + " is required"})
		}
	}
	if root["type"] != nil && root["type"] != "change_event" {
		issues = append(issues, ValidationIssue{Field: "json.type", Code: "invalid_value", Message: "type must be change_event"})
	}
	resource, _ := root["resource"].(string)
	operation, _ := root["operation"].(string)
	checkNonEmptyString(&issues, root, "event_id", "json.event_id")
	checkNonEmptyString(&issues, root, "resource_id", "json.resource_id")
	checkIntegerMin(&issues, root, "resource_version", "json.resource_version", 1)
	checkRange(&issues, root, "occurred_at", "json.occurred_at", 0, 0, false, "date-time")
	if root["metadata"] != nil {
		if _, ok := asObject(root["metadata"]); !ok {
			issues = append(issues, ValidationIssue{Field: "json.metadata", Code: "invalid_type", Message: "metadata must be an object"})
		}
	}
	if root["resource"] != nil && !in(resource, []string{"entity", "object", "task", "observation"}) {
		issues = append(issues, ValidationIssue{Field: "json.resource", Code: "invalid_value", Message: "resource must be one of entity, object, task, observation"})
	}
	if root["operation"] != nil && !in(operation, []string{"created", "updated", "deleted"}) {
		issues = append(issues, ValidationIssue{Field: "json.operation", Code: "invalid_value", Message: "operation must be one of created, updated, deleted"})
	}
	if operation == "deleted" {
		if root["snapshot"] != nil {
			issues = append(issues, ValidationIssue{Field: "json.snapshot", Code: "invalid_value", Message: "snapshot must be null for deleted events"})
		}
		return issues
	}
	snapshot, ok := asObject(root["snapshot"])
	if (operation == "created" || operation == "updated") && !ok {
		issues = append(issues, ValidationIssue{Field: "json.snapshot", Code: "invalid_type", Message: "snapshot must be an object"})
		return issues
	}
	if ok {
		issues = append(issues, validateSnapshot(resource, snapshot)...)
	}
	return issues
}

func validateSnapshot(resource string, snapshot object) []ValidationIssue {
	var issues []ValidationIssue
	checkCommon := func(allowed, required []string) {
		for key := range snapshot {
			if !in(key, allowed) {
				issues = append(issues, ValidationIssue{Field: "json.snapshot." + key, Code: "unknown_field", Message: key + " is not allowed"})
			}
		}
		for _, key := range required {
			if snapshot[key] == nil {
				issues = append(issues, ValidationIssue{Field: "json.snapshot." + key, Code: "required", Message: key + " is required"})
			}
		}
	}
	common := []string{"version", "created_at", "updated_at", "json"}
	switch resource {
	case "entity":
		checkCommon(append([]string{"entity_id", "entity_type", "subtype", "alias"}, common...), append([]string{"entity_id", "entity_type"}, common...))
		checkNonEmptyString(&issues, snapshot, "entity_id", "json.snapshot.entity_id")
		variant, _ := snapshot["entity_type"].(string)
		if !in(variant, []string{"asset", "track", "geofeature"}) {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.entity_type", Code: "invalid_value", Message: "entity_type must be one of asset, track, geofeature"})
		}
		if js, ok := asObject(snapshot["json"]); ok {
			issues = append(issues, prefix(validateEntity(js, variant), "json.snapshot.json")...)
		} else if snapshot["json"] != nil {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.json", Code: "invalid_type", Message: "json must be an object"})
		}
	case "object":
		checkCommon(append([]string{"object_id", "object_type", "owner_type", "owner_id"}, common...), append([]string{"object_id", "object_type", "owner_type", "owner_id"}, common...))
		checkNonEmptyString(&issues, snapshot, "object_id", "json.snapshot.object_id")
		variant, _ := snapshot["object_type"].(string)
		if !in(variant, []string{"log", "photo", "document"}) {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.object_type", Code: "invalid_value", Message: "object_type must be one of log, photo, document"})
		}
		ownerType, _ := snapshot["owner_type"].(string)
		if !in(ownerType, []string{"entity", "observation", "task", "system"}) {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.owner_type", Code: "invalid_value", Message: "owner_type must be one of entity, observation, task, system"})
		}
		if js, ok := asObject(snapshot["json"]); ok {
			issues = append(issues, prefix(validateObject(js, variant), "json.snapshot.json")...)
		} else if snapshot["json"] != nil {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.json", Code: "invalid_type", Message: "json must be an object"})
		}
	case "task":
		checkCommon(append([]string{"task_id", "status", "asset_id", "command_catalog_object_id"}, common...), append([]string{"task_id", "status", "asset_id", "command_catalog_object_id"}, common...))
		checkNonEmptyString(&issues, snapshot, "task_id", "json.snapshot.task_id")
		status, _ := snapshot["status"].(string)
		if !in(status, []string{"pending", "acknowledged", "completed", "failed"}) {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.status", Code: "invalid_value", Message: "status must be one of pending, acknowledged, completed, failed"})
		}
		if js, ok := asObject(snapshot["json"]); ok {
			issues = append(issues, prefix(validateTask(js), "json.snapshot.json")...)
		} else if snapshot["json"] != nil {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.json", Code: "invalid_type", Message: "json must be an object"})
		}
	case "observation":
		checkCommon(append([]string{"observation_id", "source_asset_id"}, common...), append([]string{"observation_id", "source_asset_id"}, common...))
		checkNonEmptyString(&issues, snapshot, "observation_id", "json.snapshot.observation_id")
		if js, ok := asObject(snapshot["json"]); ok {
			issues = append(issues, prefix(validateObservation(js), "json.snapshot.json")...)
		} else if snapshot["json"] != nil {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.json", Code: "invalid_type", Message: "json must be an object"})
		}
	}
	checkIntegerMin(&issues, snapshot, "version", "json.snapshot.version", 1)
	checkRange(&issues, snapshot, "created_at", "json.snapshot.created_at", 0, 0, false, "date-time")
	checkRange(&issues, snapshot, "updated_at", "json.snapshot.updated_at", 0, 0, false, "date-time")
	return issues
}

func validateCustomSection(root object) []ValidationIssue {
	var issues []ValidationIssue
	for key, value := range root {
		obj, ok := asObject(value)
		if !strings.HasPrefix(key, "custom_") {
			issues = append(issues, ValidationIssue{Field: "json." + key, Code: "unknown_field", Message: key + " is not allowed at the top level"})
		} else if !ok {
			issues = append(issues, ValidationIssue{Field: "json." + key, Code: "invalid_type", Message: key + " must be an object"})
		} else {
			issues = append(issues, collectLimitIssues("json."+key, obj, nil, 16*1024, 8, 100, 100)...)
		}
	}
	return issues
}

func validateSighting(s object, base string) []ValidationIssue {
	var issues []ValidationIssue
	kind, _ := s["kind"].(string)
	if !in(kind, []string{"line_of_bearing", "point", "area"}) {
		return []ValidationIssue{{Field: base + ".kind", Code: "invalid_value", Message: "kind must be one of line_of_bearing, point, area"}}
	}
	data, ok := asObject(s["data"])
	if !ok {
		return issues
	}
	switch kind {
	case "line_of_bearing":
		issues = append(issues, allowed(data, base+".data", []string{"observer_latitude", "observer_longitude", "observer_altitude_m", "azimuth_deg", "elevation_deg", "range_m", "uncertainty_deg"})...)
		for _, key := range []string{"observer_latitude", "observer_longitude", "azimuth_deg"} {
			if data[key] == nil {
				issues = append(issues, ValidationIssue{Field: base + ".data." + key, Code: "required", Message: key + " is required"})
			}
		}
		checkNum(&issues, data, "observer_latitude", base+".data.observer_latitude", -90, 90, false)
		checkNum(&issues, data, "observer_longitude", base+".data.observer_longitude", -180, 180, false)
		checkNum(&issues, data, "azimuth_deg", base+".data.azimuth_deg", 0, 360, true)
		checkNum(&issues, data, "elevation_deg", base+".data.elevation_deg", -90, 90, false)
		checkNum(&issues, data, "range_m", base+".data.range_m", 0, 0, false)
		checkNum(&issues, data, "uncertainty_deg", base+".data.uncertainty_deg", 0, 0, false)
	case "point":
		issues = append(issues, allowed(data, base+".data", []string{"latitude", "longitude", "altitude_m", "uncertainty_radius_m"})...)
		for _, key := range []string{"latitude", "longitude"} {
			if data[key] == nil {
				issues = append(issues, ValidationIssue{Field: base + ".data." + key, Code: "required", Message: key + " is required"})
			}
		}
		checkNum(&issues, data, "latitude", base+".data.latitude", -90, 90, false)
		checkNum(&issues, data, "longitude", base+".data.longitude", -180, 180, false)
		checkNum(&issues, data, "uncertainty_radius_m", base+".data.uncertainty_radius_m", 0, 0, false)
	case "area":
		issues = append(issues, allowed(data, base+".data", []string{"geometry", "confidence"})...)
		if geo, ok := asObject(data["geometry"]); ok {
			issues = append(issues, validateGeometry(geo, base+".data.geometry", map[string]struct{}{"Polygon": {}, "MultiPolygon": {}})...)
		} else if data["geometry"] == nil {
			issues = append(issues, ValidationIssue{Field: base + ".data.geometry", Code: "required", Message: "geometry is required"})
		}
		checkNum(&issues, data, "confidence", base+".data.confidence", 0, 1, false)
	}
	return issues
}

func validateGeometry(geo object, base string, allowedTypes map[string]struct{}) []ValidationIssue {
	var issues []ValidationIssue
	typ, _ := geo["type"].(string)
	if _, ok := allowedTypes[typ]; typ == "" || !ok {
		keys := make([]string, 0, len(allowedTypes))
		for key := range allowedTypes {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if _, ok := allowedTypes["Polygon"]; ok && len(allowedTypes) == 2 {
			keys = []string{"Polygon", "MultiPolygon"}
		} else if len(keys) == len(standardGeoJSONTypes) {
			keys = []string{"Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection"}
		}
		return []ValidationIssue{{Field: base + ".type", Code: "invalid_value", Message: "type must be one of " + strings.Join(keys, ", ")}}
	}
	if typ == "GeometryCollection" {
		arr, ok := geo["geometries"].([]any)
		if !ok {
			return []ValidationIssue{{Field: base + ".geometries", Code: "required", Message: "geometries is required"}}
		}
		for i, child := range arr {
			if obj, ok := asObject(child); ok {
				issues = append(issues, validateGeometry(obj, fmt.Sprintf("%s.geometries[%d]", base, i), standardGeoJSONTypes)...)
			}
		}
		return issues
	}
	coords := geo["coordinates"]
	if coords == nil {
		return []ValidationIssue{{Field: base + ".coordinates", Code: "required", Message: "coordinates is required"}}
	}
	switch typ {
	case "Point":
		_, issues = position(coords, base+".coordinates")
	case "LineString":
		_, issues = line(coords, base+".coordinates")
	case "Polygon":
		issues = polygon(coords, base+".coordinates")
	case "MultiPoint":
		issues = positions(coords, base+".coordinates")
	case "MultiLineString":
		issues = nested(coords, base+".coordinates", func(v any, p string) []ValidationIssue { _, iss := line(v, p); return iss })
	case "MultiPolygon":
		issues = nested(coords, base+".coordinates", polygon)
	}
	return issues
}

func collectTopLevel(root object, allowedFields []string, label string) []ValidationIssue {
	var issues []ValidationIssue
	for key := range root {
		if label != "commandCatalog" && label != "customSection" {
			if _, ok := promotedFields[key]; ok {
				issues = append(issues, ValidationIssue{Field: "json." + key, Code: "promoted_field", Message: key + " is a promoted top-level field"})
				continue
			}
		}
		if !in(key, allowedFields) && !strings.HasPrefix(key, "custom_") {
			if label == "object" && (key == "manifest" || key == "manifest_version") {
				continue
			}
			issues = append(issues, ValidationIssue{Field: "json." + key, Code: "unknown_field", Message: key + " is not allowed"})
		}
	}
	return issues
}

func collectCustom(root object, base string) []ValidationIssue {
	var issues []ValidationIssue
	for key, value := range root {
		if !strings.HasPrefix(key, "custom_") {
			continue
		}
		obj, ok := asObject(value)
		if !ok {
			issues = append(issues, ValidationIssue{Field: base + "." + key, Code: "invalid_type", Message: key + " must be an object"})
			continue
		}
		issues = append(issues, collectLimitIssues(base+"."+key, obj, nil, 16*1024, 8, 100, 100)...)
	}
	if comps, ok := asObject(root["components"]); ok {
		for key, value := range comps {
			if !strings.HasPrefix(key, "custom_") {
				continue
			}
			if obj, ok := asObject(value); ok {
				issues = append(issues, collectLimitIssues(base+".components."+key, obj, nil, 16*1024, 8, 100, 100)...)
			} else {
				issues = append(issues, ValidationIssue{Field: base + ".components." + key, Code: "invalid_type", Message: key + " must be an object"})
			}
		}
	}
	return issues
}

func collectLimitIssues(base string, value object, raw []byte, maxBytes, maxDepth, maxFields, maxKeyLength int) []ValidationIssue {
	var issues []ValidationIssue
	if raw == nil {
		raw, _ = json.Marshal(value)
	}
	if len(raw) > maxBytes {
		issues = append(issues, ValidationIssue{Field: base, Code: "limit_exceeded", Message: base + " exceeds the size limit"})
	}
	fields := 0
	var walk func(any, string, int)
	walk = func(node any, path string, depth int) {
		if depth > maxDepth {
			issues = append(issues, ValidationIssue{Field: path, Code: "limit_exceeded", Message: path + " exceeds the nesting limit"})
			return
		}
		switch n := node.(type) {
		case []any:
			for i, child := range n {
				walk(child, fmt.Sprintf("%s[%d]", path, i), depth+1)
			}
		case map[string]any:
			for key, child := range n {
				fields++
				if fields > maxFields {
					issues = append(issues, ValidationIssue{Field: path, Code: "limit_exceeded", Message: path + " exceeds the field-count limit"})
					return
				}
				if len(key) > maxKeyLength {
					issues = append(issues, ValidationIssue{Field: path + "." + key, Code: "limit_exceeded", Message: path + "." + key + " exceeds the key-length limit"})
				}
				walk(child, path+"."+key, depth+1)
			}
		}
	}
	walk(value, base, 1)
	return issues
}

func allowed(root object, base string, allowedFields []string) []ValidationIssue {
	var issues []ValidationIssue
	for key := range root {
		if !in(key, allowedFields) {
			issues = append(issues, ValidationIssue{Field: base + "." + key, Code: "unknown_field", Message: key + " is not allowed"})
		}
	}
	return issues
}

func allowedEntityComponent(variant, key string) bool {
	base := []string{"status", "custom_"}
	switch variant {
	case "asset":
		return in(key, append(base, "supported_commands", "telemetry", "heartbeat", "health", "communications", "sensor_refs"))
	case "track":
		return in(key, append(base, "telemetry", "fusion_summary"))
	case "geofeature":
		return in(key, append(base, "geometry"))
	default:
		return true
	}
}

func asObject(value any) (object, bool) {
	obj, ok := value.(map[string]any)
	return obj, ok
}

func in(value string, list []string) bool {
	for _, item := range list {
		if value == item {
			return true
		}
	}
	return false
}

func dedupe(issues []ValidationIssue) []ValidationIssue {
	seen := map[string]struct{}{}
	var out []ValidationIssue
	for _, issue := range issues {
		key := issue.Field + "|" + issue.Code + "|" + issue.Message
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, issue)
	}
	return out
}

func prefix(issues []ValidationIssue, base string) []ValidationIssue {
	out := make([]ValidationIssue, len(issues))
	for i, issue := range issues {
		out[i] = issue
		if issue.Field == "json" {
			out[i].Field = base
		} else {
			out[i].Field = strings.Replace(issue.Field, "json", base, 1)
		}
	}
	return out
}

func checkRange(issues *[]ValidationIssue, root object, key, field string, min, max float64, exclusive bool, kind string) {
	value, ok := root[key]
	if !ok {
		return
	}
	if kind == "date-time" {
		text, ok := value.(string)
		if !ok {
			*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_type", Message: key + " must be a string"})
			return
		}
		if _, err := time.Parse(time.RFC3339, text); err != nil {
			*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_value", Message: key + " must match date-time format"})
		}
		return
	}
	checkNum(issues, root, key, field, min, max, exclusive)
}

func checkNonEmptyString(issues *[]ValidationIssue, root object, key, field string) {
	value, ok := root[key]
	if !ok {
		return
	}
	text, ok := value.(string)
	if !ok {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_type", Message: key + " must be a string"})
		return
	}
	if text == "" {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_value", Message: key + " must not be empty"})
	}
}

func checkIntegerMin(issues *[]ValidationIssue, root object, key, field string, min int) {
	value, ok := root[key]
	if !ok {
		return
	}
	num, ok := value.(float64)
	if !ok || math.Trunc(num) != num {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_type", Message: key + " must be an integer"})
		return
	}
	if num < float64(min) {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_value", Message: key + " is out of range"})
	}
}

func checkNum(issues *[]ValidationIssue, root object, key, field string, min, max float64, exclusive bool) {
	value, ok := root[key]
	if !ok {
		return
	}
	num, ok := value.(float64)
	if !ok {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_type", Message: key + " must be a number"})
		return
	}
	if num < min || (max != 0 && (num > max || (exclusive && num >= max))) {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_value", Message: key + " is out of range"})
	}
}

func position(value any, field string) (coord, []ValidationIssue) {
	arr, ok := value.([]any)
	if !ok || len(arr) < 2 || len(arr) > 3 {
		return coord{}, []ValidationIssue{{Field: field, Code: "invalid_value", Message: "position must be [longitude, latitude] or [longitude, latitude, altitude_m]"}}
	}
	lon, okLon := arr[0].(float64)
	lat, okLat := arr[1].(float64)
	if !okLon || lon < -180 || lon > 180 {
		return coord{}, []ValidationIssue{{Field: field + "[0]", Code: "invalid_value", Message: "longitude is out of range"}}
	}
	if !okLat || lat < -90 || lat > 90 {
		return coord{}, []ValidationIssue{{Field: field + "[1]", Code: "invalid_value", Message: "latitude is out of range"}}
	}
	return coord{lon, lat}, nil
}

func positions(value any, field string) []ValidationIssue {
	arr, ok := value.([]any)
	if !ok {
		return []ValidationIssue{{Field: field, Code: "invalid_type", Message: "coordinates must be an array"}}
	}
	var issues []ValidationIssue
	for i, child := range arr {
		_, iss := position(child, fmt.Sprintf("%s[%d]", field, i))
		issues = append(issues, iss...)
	}
	return issues
}

func line(value any, field string) ([]coord, []ValidationIssue) {
	arr, ok := value.([]any)
	if !ok {
		return nil, []ValidationIssue{{Field: field, Code: "invalid_type", Message: "coordinates must be an array"}}
	}
	var out []coord
	var issues []ValidationIssue
	for i, child := range arr {
		pos, iss := position(child, fmt.Sprintf("%s[%d]", field, i))
		issues = append(issues, iss...)
		if len(iss) == 0 {
			out = append(out, pos)
		}
	}
	if len(out) > 0 && len(out) < 2 {
		issues = append(issues, ValidationIssue{Field: field, Code: "invalid_value", Message: "LineString must contain at least 2 positions"})
	}
	return out, issues
}

func polygon(value any, field string) []ValidationIssue {
	rings, ok := value.([]any)
	if !ok {
		return []ValidationIssue{{Field: field, Code: "invalid_type", Message: "coordinates must be an array"}}
	}
	var issues []ValidationIssue
	for i, rawRing := range rings {
		ringField := fmt.Sprintf("%s[%d]", field, i)
		ring, iss := line(rawRing, ringField)
		issues = append(issues, iss...)
		if len(ring) > 0 && len(ring) < 4 {
			issues = append(issues, ValidationIssue{Field: ringField, Code: "invalid_value", Message: "Polygon ring must contain at least 4 positions"})
		}
		if len(ring) >= 2 && ring[0] != ring[len(ring)-1] {
			issues = append(issues, ValidationIssue{Field: ringField, Code: "invalid_value", Message: "Polygon ring must be closed"})
		}
		if selfIntersects(ring) {
			issues = append(issues, ValidationIssue{Field: ringField, Code: "invalid_value", Message: "Polygon ring must not self-intersect"})
		}
	}
	return issues
}

func nested(value any, field string, fn func(any, string) []ValidationIssue) []ValidationIssue {
	arr, ok := value.([]any)
	if !ok {
		return []ValidationIssue{{Field: field, Code: "invalid_type", Message: "coordinates must be an array"}}
	}
	var issues []ValidationIssue
	for i, child := range arr {
		issues = append(issues, fn(child, fmt.Sprintf("%s[%d]", field, i))...)
	}
	return issues
}

func selfIntersects(ring []coord) bool {
	if len(ring) < 4 {
		return false
	}
	for i := 0; i < len(ring)-1; i++ {
		for j := i + 1; j < len(ring)-1; j++ {
			if int(math.Abs(float64(i-j))) <= 1 || (i == 0 && j == len(ring)-2) {
				continue
			}
			if segmentsIntersect(ring[i], ring[i+1], ring[j], ring[j+1]) {
				return true
			}
		}
	}
	return false
}

func segmentsIntersect(a, b, c, d coord) bool {
	orient := func(p, q, r coord) float64 {
		return (q[0]-p[0])*(r[1]-p[1]) - (q[1]-p[1])*(r[0]-p[0])
	}
	return math.Signbit(orient(a, b, c)) != math.Signbit(orient(a, b, d)) &&
		math.Signbit(orient(c, d, a)) != math.Signbit(orient(c, d, b))
}
