package protocol

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type jsonObject = map[string]any

var promotedFields = map[string]struct{}{
	"entity_id": {}, "object_id": {}, "task_id": {}, "observation_id": {},
	"type": {}, "status": {}, "owner_type": {}, "owner_id": {}, "asset_id": {},
	"source_asset_id": {}, "command_catalog_object_id": {}, "created_at": {},
	"updated_at": {}, "version": {}, "target_entity_id": {}, "observed_at": {},
}

var standardGeoJSONTypeOrder = []string{
	"Point",
	"MultiPoint",
	"LineString",
	"MultiLineString",
	"Polygon",
	"MultiPolygon",
	"GeometryCollection",
}

var standardGeoJSONTypes = stringSet(standardGeoJSONTypeOrder)

const (
	rootMaxBytes       = 64 * 1024
	rootMaxDepth       = 16
	rootMaxFields      = 500
	rootMaxKeyLength   = 100
	customMaxBytes     = 16 * 1024
	customMaxDepth     = 8
	customMaxFields    = 100
	customMaxKeyLength = 100
)

func validateEntityPre(root jsonObject, variant string) []ValidationIssue {
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
	if hasComponents {
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

func (v *Validator) validateEntityWithSchema(root jsonObject, variant string) []ValidationIssue {
	issues := validateEntityPre(root, variant)
	if variant == "" {
		return dedupe(issues)
	}
	promoted := fieldSetByCode(issues, "promoted_field")
	customReq := fieldSetByCode(issues, "required")
	reqPair := fieldSetByCode(issues, "required_pair")
	schemaIssues := v.runSchemaURL(entityWrapURL(variant), root)
	schemaIssues = filterSchemaEntity(schemaIssues, promoted, customReq, reqPair)
	issues = append(issues, schemaIssues...)
	return dedupe(issues)
}

func entityWrapURL(variant string) string {
	return "atlas://entity/" + variant
}

func filterSchemaEntity(schema []ValidationIssue, promoted, customReq, reqPair map[string]struct{}) []ValidationIssue {
	var out []ValidationIssue
	for _, s := range schema {
		if s.Code == "unknown_field" {
			if _, ok := promoted[s.Field]; ok {
				continue
			}
		}
		if s.Code == "required" {
			if _, ok := customReq[s.Field]; ok {
				continue
			}
			if _, ok := reqPair[s.Field]; ok {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

func validateTaskPre(root jsonObject) []ValidationIssue {
	issues := collectTopLevel(root, []string{"description", "created_by", "components", "extra"}, "task")
	issues = append(issues, collectCustom(root, "json")...)
	components, ok := asObject(root["components"])
	if root["components"] != nil && !ok {
		return append(issues, ValidationIssue{Field: "json.components", Code: "invalid_type", Message: "components must be an object"})
	}
	if !ok || !asObjectExists(components["command"]) {
		issues = append(issues, ValidationIssue{Field: "json.components.command.type", Code: "required", Message: "command.type is required"})
	} else if cmd, ok := asObject(components["command"]); ok {
		if s, _ := cmd["type"].(string); s == "" {
			issues = append(issues, ValidationIssue{Field: "json.components.command.type", Code: "required", Message: "command.type is required"})
		}
	}
	return issues
}

func asObjectExists(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func (v *Validator) validateTaskWithSchema(root jsonObject) []ValidationIssue {
	issues := validateTaskPre(root)
	promoted := fieldSetByCode(issues, "promoted_field")
	customReq := fieldSetByCode(issues, "required")
	schemaIssues := v.runSchemaURL("task.schema.json", root)
	for _, s := range schemaIssues {
		if s.Code == "unknown_field" {
			if _, ok := promoted[s.Field]; ok {
				continue
			}
		}
		if s.Code == "required" {
			if _, ok := customReq[s.Field]; ok {
				continue
			}
		}
		issues = append(issues, s)
	}
	return dedupe(issues)
}

func validateObservationPre(root jsonObject) []ValidationIssue {
	issues := collectTopLevel(root, []string{"state", "latest_sighting", "sightings_object_id", "extra"}, "observation")
	issues = append(issues, collectCustom(root, "json")...)
	if _, ok := root["state"]; !ok {
		issues = append(issues, ValidationIssue{Field: "json.state", Code: "required", Message: "state is required"})
	} else if _, ok := root["state"].(string); !ok {
		issues = append(issues, ValidationIssue{Field: "json.state", Code: "invalid_type", Message: "state must be a string"})
	} else if state, _ := root["state"].(string); !in(state, []string{"active", "inactive", "ended"}) {
		issues = append(issues, ValidationIssue{Field: "json.state", Code: "invalid_value", Message: "state must be one of active, inactive, ended"})
	}
	if sighting, ok := asObject(root["latest_sighting"]); ok {
		issues = append(issues, validateLatestSighting(sighting, "json.latest_sighting")...)
	}
	return issues
}

func (v *Validator) validateObservationWithSchema(root jsonObject) []ValidationIssue {
	issues := validateObservationPre(root)
	promoted := fieldSetByCode(issues, "promoted_field")
	customReq := fieldSetByCode(issues, "required")
	schemaIssues := v.runSchemaURL("observation.schema.json", root)
	for _, s := range schemaIssues {
		if s.Code == "unknown_field" {
			if _, ok := promoted[s.Field]; ok {
				continue
			}
		}
		if s.Code == "required" {
			if _, ok := customReq[s.Field]; ok {
				continue
			}
		}
		issues = append(issues, s)
	}
	return dedupe(issues)
}

func objectWrapURL(variant string) string {
	return "atlas://object/" + variant
}

func validateObjectPre(root jsonObject, variant string) []ValidationIssue {
	allowed := map[string][]string{
		"log":                 {"log_type", "started_at", "ended_at", "extra"},
		"photo":               {"content_type", "captured_at", "width_px", "height_px", "extra"},
		"document":            {"content_type", "extra"},
		"observation_history": {"format_version", "extra"},
		"track_provenance":    {"format_version", "extra"},
	}
	allowedFields := allowed[variant]
	if allowedFields == nil {
		allowedFields = []string{}
	}
	issues := collectTopLevel(root, allowedFields, "object")
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

func (v *Validator) validateObjectWithSchema(root jsonObject, variant string) []ValidationIssue {
	issues := validateObjectPre(root, variant)
	if variant == "" {
		return dedupe(issues)
	}
	promoted := fieldSetByCode(issues, "promoted_field")
	reserved := fieldSetByCode(issues, "reserved_field")
	customReq := fieldSetByCode(issues, "required")
	schemaIssues := v.runSchemaURL(objectWrapURL(variant), root)
	for _, s := range schemaIssues {
		if s.Code == "unknown_field" {
			if _, ok := promoted[s.Field]; ok {
				continue
			}
			if _, ok := reserved[s.Field]; ok {
				continue
			}
		}
		if s.Code == "required" {
			if _, ok := customReq[s.Field]; ok {
				continue
			}
		}
		issues = append(issues, s)
	}
	return dedupe(issues)
}

func (v *Validator) validateCommandCatalogWithSchema(root jsonObject) []ValidationIssue {
	issues := collectTopLevel(root, []string{"type", "name", "description", "commands"}, "commandCatalog")
	typoPaths := map[string]struct{}{}
	if arr, ok := root["commands"].([]any); ok {
		seen := map[string]int{}
		for i, raw := range arr {
			cmd, ok := asObject(raw)
			if !ok {
				continue
			}
			if _, has := cmd["parameter_schema"]; has {
				f := fmt.Sprintf("json.commands[%d].parameter_schema", i)
				typoPaths[f] = struct{}{}
				issues = append(issues, ValidationIssue{
					Field:   f,
					Code:    "unknown_field",
					Message: `"parameter_schema" is not allowed; use "parameters_schema"`,
				})
			}
			if id, ok := cmd["id"].(string); ok {
				if _, exists := seen[id]; exists {
					issues = append(issues, ValidationIssue{
						Field:   fmt.Sprintf("json.commands[%d].id", i),
						Code:    "duplicate_command_id",
						Message: fmt.Sprintf("command id %q must be unique", id),
					})
				} else {
					seen[id] = i
				}
			}
		}
	}
	schemaIssues := v.runSchemaURL("command-catalog.schema.json", root)
	for _, s := range schemaIssues {
		if s.Code == "unknown_field" {
			if _, ok := typoPaths[s.Field]; ok {
				continue
			}
		}
		issues = append(issues, s)
	}
	return dedupe(issues)
}

func (v *Validator) validateChangeEventWithSchema(root jsonObject) []ValidationIssue {
	issues := v.runSchemaURL("change-event.schema.json", root)
	operation, _ := root["operation"].(string)
	resource, _ := root["resource"].(string)
	snapshot := root["snapshot"]
	if operation == "deleted" {
		if snapshot != nil {
			issues = append(issues, ValidationIssue{Field: "json.snapshot", Code: "invalid_value", Message: "snapshot must be null for deleted events"})
		}
		return dedupe(issues)
	}
	if operation == "created" || operation == "updated" {
		snap, ok := asObject(snapshot)
		if !ok {
			issues = append(issues, ValidationIssue{Field: "json.snapshot", Code: "invalid_type", Message: "snapshot must be an object"})
			return dedupe(issues)
		}
		inferred := inferSnapshotResource(snap)
		if inferred != "" && inferred != resource {
			issues = append(issues, ValidationIssue{
				Field:   "json.snapshot",
				Code:    "invalid_value",
				Message: fmt.Sprintf("snapshot resource type %s does not match declared resource %s", inferred, resource),
			})
			return dedupe(issues)
		}
		if inferred == "" {
			inferred = resource
		}
		issues = append(issues, v.validateSnapshot(inferred, snap)...)
		if field, value, ok := snapshotResourceIdentity(inferred, snap); ok {
			if resourceID, ok := root["resource_id"].(string); ok && resourceID != value {
				issues = append(issues, ValidationIssue{
					Field:   "json.resource_id",
					Code:    "invalid_value",
					Message: "resource_id must match snapshot " + field,
				})
			}
		}
		if resourceVersion, ok := integerValue(root["resource_version"]); ok {
			if snapshotVersion, ok := integerValue(snap["version"]); ok && resourceVersion != snapshotVersion {
				issues = append(issues, ValidationIssue{
					Field:   "json.resource_version",
					Code:    "invalid_value",
					Message: "resource_version must match snapshot version",
				})
			}
		}
	}
	return dedupe(issues)
}

func inferSnapshotResource(snapshot jsonObject) string {
	if snapshot["entity_id"] != nil || snapshot["entity_type"] != nil {
		return "entity"
	}
	if snapshot["object_id"] != nil || snapshot["object_type"] != nil {
		return "object"
	}
	if snapshot["task_id"] != nil {
		return "task"
	}
	if snapshot["observation_id"] != nil {
		return "observation"
	}
	return ""
}

func snapshotResourceIdentity(resource string, snapshot jsonObject) (string, string, bool) {
	fieldByResource := map[string]string{
		"entity":      "entity_id",
		"object":      "object_id",
		"task":        "task_id",
		"observation": "observation_id",
	}
	field, ok := fieldByResource[resource]
	if !ok {
		return "", "", false
	}
	value, ok := snapshot[field].(string)
	if !ok || value == "" {
		return "", "", false
	}
	return field, value, true
}

func integerValue(value any) (int64, bool) {
	number, ok := value.(float64)
	if !ok || math.Trunc(number) != number {
		return 0, false
	}
	return int64(number), true
}

func (v *Validator) validateSnapshot(resource string, snapshot jsonObject) []ValidationIssue {
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
		if ver := snapshot["version"]; ver != nil {
			num, ok := ver.(float64)
			if !ok || math.Trunc(num) != num || num < 1 {
				issues = append(issues, ValidationIssue{Field: "json.snapshot.version", Code: "invalid_value", Message: "version is out of range"})
			}
		}
		for _, key := range []string{"created_at", "updated_at"} {
			if v := snapshot[key]; v != nil {
				if _, ok := v.(string); !ok {
					issues = append(issues, ValidationIssue{Field: "json.snapshot." + key, Code: "invalid_type", Message: key + " must be a string"})
				}
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
			issues = append(issues, prefixIssues(v.validateEntityWithSchema(js, variant), "json.snapshot.json")...)
		} else if snapshot["json"] != nil {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.json", Code: "invalid_type", Message: "json must be an object"})
		}
	case "object":
		checkCommon(append([]string{"object_id", "object_type", "owner_type", "owner_id"}, common...), append([]string{"object_id", "object_type", "owner_type", "owner_id"}, common...))
		checkNonEmptyString(&issues, snapshot, "object_id", "json.snapshot.object_id")
		variant, _ := snapshot["object_type"].(string)
		if !in(variant, []string{"log", "photo", "document", "observation_history", "track_provenance"}) {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.object_type", Code: "invalid_value", Message: "object_type must be one of log, photo, document, observation_history, track_provenance"})
		}
		ownerType, _ := snapshot["owner_type"].(string)
		if !in(ownerType, []string{"entity", "observation", "task", "system"}) {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.owner_type", Code: "invalid_value", Message: "owner_type must be one of entity, observation, task, system"})
		}
		if js, ok := asObject(snapshot["json"]); ok {
			issues = append(issues, prefixIssues(v.validateObjectWithSchema(js, variant), "json.snapshot.json")...)
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
			issues = append(issues, prefixIssues(v.validateTaskWithSchema(js), "json.snapshot.json")...)
		} else if snapshot["json"] != nil {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.json", Code: "invalid_type", Message: "json must be an object"})
		}
	case "observation":
		checkCommon(append([]string{"observation_id", "source_asset_id"}, common...), append([]string{"observation_id", "source_asset_id"}, common...))
		checkNonEmptyString(&issues, snapshot, "observation_id", "json.snapshot.observation_id")
		if js, ok := asObject(snapshot["json"]); ok {
			issues = append(issues, prefixIssues(v.validateObservationWithSchema(js), "json.snapshot.json")...)
		} else if snapshot["json"] != nil {
			issues = append(issues, ValidationIssue{Field: "json.snapshot.json", Code: "invalid_type", Message: "json must be an object"})
		}
	default:
		return issues
	}
	return issues
}

func validateCustomSection(root jsonObject) []ValidationIssue {
	var issues []ValidationIssue
	for key, value := range root {
		obj, ok := asObject(value)
		if !strings.HasPrefix(key, "custom_") {
			issues = append(issues, ValidationIssue{Field: "json." + key, Code: "unknown_field", Message: key + " is not allowed at the top level"})
		} else if !ok {
			issues = append(issues, ValidationIssue{Field: "json." + key, Code: "invalid_type", Message: key + " must be an object"})
		} else {
			issues = append(issues, collectLimitIssues("json."+key, obj, nil, customMaxBytes, customMaxDepth, customMaxFields, customMaxKeyLength)...)
		}
	}
	return issues
}

func fieldSetByCode(issues []ValidationIssue, code string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, i := range issues {
		if i.Code == code {
			out[i.Field] = struct{}{}
		}
	}
	return out
}

func collectTopLevel(root jsonObject, allowedFields []string, label string) []ValidationIssue {
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

func collectCustom(root jsonObject, base string) []ValidationIssue {
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
		issues = append(issues, collectLimitIssues(base+"."+key, obj, nil, customMaxBytes, customMaxDepth, customMaxFields, customMaxKeyLength)...)
	}
	if comps, ok := asObject(root["components"]); ok {
		for key, value := range comps {
			if !strings.HasPrefix(key, "custom_") {
				continue
			}
			if obj, ok := asObject(value); ok {
				issues = append(issues, collectLimitIssues(base+".components."+key, obj, nil, customMaxBytes, customMaxDepth, customMaxFields, customMaxKeyLength)...)
			} else {
				issues = append(issues, ValidationIssue{Field: base + ".components." + key, Code: "invalid_type", Message: key + " must be an object"})
			}
		}
	}
	return issues
}

func collectLimitIssues(base string, value jsonObject, raw []byte, maxBytes, maxDepth, maxFields, maxKeyLength int) []ValidationIssue {
	var issues []ValidationIssue
	if raw == nil {
		raw, _ = json.Marshal(value)
	}
	if len(raw) > maxBytes {
		issues = append(issues, ValidationIssue{Field: base, Code: "limit_exceeded", Message: base + " exceeds the size limit"})
	}
	if countFields(value) > maxFields {
		issues = append(issues, ValidationIssue{Field: base, Code: "limit_exceeded", Message: base + " exceeds the field-count limit"})
	}
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

func validateLatestSighting(sighting jsonObject, base string) []ValidationIssue {
	var issues []ValidationIssue
	kind, _ := sighting["kind"].(string)
	if kind == "" || !in(kind, []string{"line_of_bearing", "point", "area"}) {
		return []ValidationIssue{{Field: base + ".kind", Code: "invalid_value", Message: "kind must be one of line_of_bearing, point, area"}}
	}
	data, ok := asObject(sighting["data"])
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
		checkNumberRange(&issues, data, "observer_latitude", base+".data.observer_latitude", -90, 90, false)
		checkNumberRange(&issues, data, "observer_longitude", base+".data.observer_longitude", -180, 180, false)
		checkNumberRange(&issues, data, "azimuth_deg", base+".data.azimuth_deg", 0, 360, true)
		checkNumberRange(&issues, data, "elevation_deg", base+".data.elevation_deg", -90, 90, false)
		checkNumberMin(&issues, data, "range_m", base+".data.range_m", 0)
		checkNumberMin(&issues, data, "uncertainty_deg", base+".data.uncertainty_deg", 0)
	case "point":
		issues = append(issues, allowed(data, base+".data", []string{"latitude", "longitude", "altitude_m", "uncertainty_radius_m"})...)
		for _, key := range []string{"latitude", "longitude"} {
			if data[key] == nil {
				issues = append(issues, ValidationIssue{Field: base + ".data." + key, Code: "required", Message: key + " is required"})
			}
		}
		checkNumberRange(&issues, data, "latitude", base+".data.latitude", -90, 90, false)
		checkNumberRange(&issues, data, "longitude", base+".data.longitude", -180, 180, false)
		checkNumberMin(&issues, data, "uncertainty_radius_m", base+".data.uncertainty_radius_m", 0)
	case "area":
		issues = append(issues, allowed(data, base+".data", []string{"geometry", "confidence"})...)
		if geo, ok := asObject(data["geometry"]); ok {
			issues = append(issues, validateGeometry(geo, base+".data.geometry", map[string]struct{}{"Polygon": {}, "MultiPolygon": {}})...)
		} else if data["geometry"] == nil {
			issues = append(issues, ValidationIssue{Field: base + ".data.geometry", Code: "required", Message: "geometry is required"})
		} else {
			issues = append(issues, ValidationIssue{Field: base + ".data.geometry", Code: "invalid_type", Message: "geometry must be an object"})
		}
		checkNumberRange(&issues, data, "confidence", base+".data.confidence", 0, 1, false)
	}
	return issues
}

func validateGeometry(geo jsonObject, base string, allowedTypes map[string]struct{}) []ValidationIssue {
	var issues []ValidationIssue
	typ, _ := geo["type"].(string)
	if _, ok := allowedTypes[typ]; typ == "" || !ok {
		keys := orderedGeometryTypes(allowedTypes)
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
			} else {
				issues = append(issues, ValidationIssue{Field: fmt.Sprintf("%s.geometries[%d]", base, i), Code: "invalid_type", Message: "geometry must be an object"})
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
		_, iss := position(coords, base+".coordinates")
		issues = append(issues, iss...)
	case "LineString":
		lineCoords, iss := line(coords, base+".coordinates")
		issues = append(issues, iss...)
		issues = append(issues, zeroLengthSegmentIssue(lineCoords, base+".coordinates")...)
	case "Polygon":
		issues = polygon(coords, base+".coordinates")
	case "MultiPoint":
		issues = positions(coords, base+".coordinates")
	case "MultiLineString":
		issues = nested(coords, base+".coordinates", func(v any, p string) []ValidationIssue {
			lineCoords, iss := line(v, p)
			return append(iss, zeroLengthSegmentIssue(lineCoords, p)...)
		})
	case "MultiPolygon":
		issues = nested(coords, base+".coordinates", polygon)
	}
	return issues
}

func allowed(root jsonObject, base string, allowedFields []string) []ValidationIssue {
	var issues []ValidationIssue
	for key := range root {
		if !in(key, allowedFields) {
			issues = append(issues, ValidationIssue{Field: base + "." + key, Code: "unknown_field", Message: key + " is not allowed"})
		}
	}
	return issues
}

func asObject(value any) (jsonObject, bool) {
	obj, ok := value.(map[string]any)
	return obj, ok
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func orderedGeometryTypes(allowedTypes map[string]struct{}) []string {
	keys := make([]string, 0, len(allowedTypes))
	seen := make(map[string]struct{}, len(allowedTypes))
	for _, key := range standardGeoJSONTypeOrder {
		if _, ok := allowedTypes[key]; ok {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	extraKeys := make([]string, 0, len(allowedTypes)-len(keys))
	for key := range allowedTypes {
		if _, ok := seen[key]; ok {
			continue
		}
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)
	return append(keys, extraKeys...)
}

func countFields(node any) int {
	switch value := node.(type) {
	case []any:
		total := 0
		for _, child := range value {
			total += countFields(child)
		}
		return total
	case map[string]any:
		total := 0
		for _, child := range value {
			total++
			total += countFields(child)
		}
		return total
	default:
		return 0
	}
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

func prefixIssues(issues []ValidationIssue, basePath string) []ValidationIssue {
	out := make([]ValidationIssue, len(issues))
	for i, issue := range issues {
		out[i] = issue
		if issue.Field == "json" {
			out[i].Field = basePath
		} else if strings.HasPrefix(issue.Field, "json.") || strings.HasPrefix(issue.Field, "json[") {
			out[i].Field = basePath + issue.Field[len("json"):]
		}
	}
	return out
}

func checkNonEmptyString(issues *[]ValidationIssue, root jsonObject, key, field string) {
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

func checkNumberMin(issues *[]ValidationIssue, root jsonObject, key, field string, min float64) {
	value, ok := root[key]
	if !ok {
		return
	}
	num, ok := value.(float64)
	if !ok {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_type", Message: key + " must be a number"})
		return
	}
	if num < min {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_value", Message: key + " is out of range"})
	}
}

func checkNumberRange(issues *[]ValidationIssue, root jsonObject, key, field string, min, max float64, exclusive bool) {
	value, ok := root[key]
	if !ok {
		return
	}
	num, ok := value.(float64)
	if !ok {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_type", Message: key + " must be a number"})
		return
	}
	if num < min || num > max || (exclusive && num >= max) {
		*issues = append(*issues, ValidationIssue{Field: field, Code: "invalid_value", Message: key + " is out of range"})
	}
}

type coord [2]float64

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
	if len(arr) == 3 {
		if _, okAlt := arr[2].(float64); !okAlt {
			return coord{}, []ValidationIssue{{Field: field + "[2]", Code: "invalid_type", Message: "altitude_m must be a number"}}
		}
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

func zeroLengthSegmentIssue(line []coord, field string) []ValidationIssue {
	for i := 1; i < len(line); i++ {
		if sameHorizontalCoord(line[i], line[i-1]) {
			return []ValidationIssue{{
				Field:   fmt.Sprintf("%s[%d]", field, i),
				Code:    "invalid_value",
				Message: "LineString must not contain zero-length segments",
			}}
		}
	}
	return nil
}

func sameHorizontalCoord(a, b coord) bool {
	return a[0] == b[0] && a[1] == b[1]
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
	orient := func(p, q, r coord) int {
		value := (q[0]-p[0])*(r[1]-p[1]) - (q[1]-p[1])*(r[0]-p[0])
		switch {
		case value > 0:
			return 1
		case value < 0:
			return -1
		default:
			return 0
		}
	}
	onSegment := func(p, q, r coord) bool {
		return q[0] <= math.Max(p[0], r[0]) && q[0] >= math.Min(p[0], r[0]) &&
			q[1] <= math.Max(p[1], r[1]) && q[1] >= math.Min(p[1], r[1])
	}
	o1 := orient(a, b, c)
	o2 := orient(a, b, d)
	o3 := orient(c, d, a)
	o4 := orient(c, d, b)
	if o1 != o2 && o3 != o4 {
		return true
	}
	if o1 == 0 && onSegment(a, c, b) {
		return true
	}
	if o2 == 0 && onSegment(a, d, b) {
		return true
	}
	if o3 == 0 && onSegment(c, a, d) {
		return true
	}
	return o4 == 0 && onSegment(c, b, d)
}
