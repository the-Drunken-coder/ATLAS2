package blob

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	maxJSONBlobSize   = 64 * 1024
	maxJSONDepth      = 16
	maxJSONFields     = 500
	maxJSONKeyLength  = 100
	maxCustomBlobSize = 16 * 1024
	maxCustomDepth    = 8
	maxCustomFields   = 100
	maxCustomKeyLen   = 100
)

type validationLimits struct {
	maxSize      int
	maxDepth     int
	maxFields    int
	maxKeyLength int
}

var (
	commonLimits   = validationLimits{maxSize: maxJSONBlobSize, maxDepth: maxJSONDepth, maxFields: maxJSONFields, maxKeyLength: maxJSONKeyLength}
	customLimits   = validationLimits{maxSize: maxCustomBlobSize, maxDepth: maxCustomDepth, maxFields: maxCustomFields, maxKeyLength: maxCustomKeyLen}
	promotedFields = map[string]struct{}{
		"entity_id": {}, "object_id": {}, "task_id": {}, "observation_id": {},
		"type": {}, "status": {}, "owner_type": {}, "owner_id": {}, "asset_id": {},
		"source_asset_id": {}, "command_catalog_object_id": {}, "created_at": {},
		"updated_at": {}, "version": {},
	}
	coreCustomSectionKeys = map[string]struct{}{
		"components": {}, "extra": {}, "description": {}, "created_by": {},
		"state": {}, "latest_sighting": {}, "sightings_object_id": {},
		"log_type": {}, "started_at": {}, "ended_at": {}, "content_type": {},
		"captured_at": {}, "width_px": {}, "height_px": {}, "manifest": {},
		"manifest_version":   {},
		"supported_commands": {}, "telemetry": {}, "geometry": {}, "heartbeat": {},
		"health": {}, "communications": {}, "sensor_refs": {}, "fusion_summary": {},
		"command": {}, "parameters": {}, "progress": {}, "result": {}, "error": {},
	}
)

func parseAndNormalize(raw []byte) (map[string]any, []Violation, error) {
	if raw == nil {
		raw = []byte("{}")
	}
	if len(raw) > maxJSONBlobSize {
		return nil, nil, &ValidationError{Violations: []Violation{{Field: "json", Code: "TOO_LARGE", Message: fmt.Sprintf("must be %d bytes or fewer", maxJSONBlobSize)}}}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, nil, &ValidationError{Violations: []Violation{{Field: "json", Code: "INVALID_JSON", Message: "must be valid JSON"}}}
	}
	if value == nil {
		return nil, nil, &ValidationError{Violations: []Violation{{Field: "json", Code: "INVALID_TYPE", Message: "must not be null"}}}
	}
	root, ok := value.(map[string]any)
	if !ok || root == nil {
		return nil, nil, &ValidationError{Violations: []Violation{{Field: "json", Code: "INVALID_TYPE", Message: "must be a JSON object"}}}
	}
	violations := validateValueLimits(root, "json", commonLimits)
	return root, violations, nil
}

func canonicalJSON(root map[string]any) ([]byte, error) {
	return json.Marshal(root)
}

func validateValueLimits(value any, path string, limits validationLimits) []Violation {
	violations := make([]Violation, 0)
	if limits.maxSize > 0 {
		data, err := json.Marshal(value)
		if err != nil {
			return []Violation{{Field: path, Code: "INVALID_JSON", Message: "must be valid JSON"}}
		}
		if len(data) > limits.maxSize {
			violations = append(violations, Violation{Field: path, Code: "TOO_LARGE", Message: fmt.Sprintf("must be %d bytes or fewer", limits.maxSize)})
		}
	}
	var fieldCount int
	walkJSON(value, path, 1, limits, &fieldCount, &violations)
	return violations
}

func walkJSON(value any, path string, depth int, limits validationLimits, fieldCount *int, violations *[]Violation) {
	if depth > limits.maxDepth {
		appendViolation(violations, path, "TOO_DEEP", fmt.Sprintf("must not exceed %d levels", limits.maxDepth))
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		*fieldCount += len(typed)
		if *fieldCount > limits.maxFields {
			appendViolation(violations, path, "TOO_MANY_FIELDS", fmt.Sprintf("must not contain more than %d object fields", limits.maxFields))
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if len(key) > limits.maxKeyLength {
				appendViolation(violations, joinPath(path, key), "KEY_TOO_LONG", fmt.Sprintf("key length must be %d characters or fewer", limits.maxKeyLength))
			}
			walkJSON(typed[key], joinPath(path, key), depth+1, limits, fieldCount, violations)
		}
	case []any:
		for idx, item := range typed {
			walkJSON(item, fmt.Sprintf("%s[%d]", path, idx), depth+1, limits, fieldCount, violations)
		}
	}
}

func appendViolation(violations *[]Violation, field, code, message string) {
	*violations = append(*violations, Violation{Field: field, Code: code, Message: message})
}

func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func isCustomKey(key string) bool {
	return strings.HasPrefix(key, "custom_")
}

func allowedTopLevelKey(key string, allowed map[string]struct{}) bool {
	if _, ok := allowed[key]; ok {
		return true
	}
	return isCustomKey(key)
}

func requireObjectField(obj map[string]any, key, path string, violations *[]Violation) map[string]any {
	value, ok := obj[key]
	if !ok {
		appendViolation(violations, path, "REQUIRED", "is required")
		return nil
	}
	child, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return nil
	}
	return child
}

// requireObjectFieldOrEmpty records REQUIRED when key is missing and returns an empty map so
// validation can accumulate further violations. It returns nil only when the value is present but not an object.
func requireObjectFieldOrEmpty(obj map[string]any, key, path string, violations *[]Violation) map[string]any {
	value, ok := obj[key]
	if !ok {
		appendViolation(violations, path, "REQUIRED", "is required")
		return map[string]any{}
	}
	child, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return nil
	}
	return child
}

func ensureObjectField(obj map[string]any, key string, violations *[]Violation) map[string]any {
	if value, ok := obj[key]; ok {
		child, ok := value.(map[string]any)
		if !ok {
			appendViolation(violations, joinPath("json", key), "INVALID_TYPE", "must be an object")
			return nil
		}
		return child
	}
	child := map[string]any{}
	obj[key] = child
	return child
}

func validateAllowedTopLevelKeys(root map[string]any, allowed map[string]struct{}, violations *[]Violation) {
	keys := make([]string, 0, len(root))
	for key := range root {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := promotedFields[key]; ok {
			appendViolation(violations, joinPath("json", key), "PROMOTED_FIELD", "must not duplicate a promoted field")
			continue
		}
		if !allowedTopLevelKey(key, allowed) {
			appendViolation(violations, joinPath("json", key), "UNKNOWN_FIELD", "is not allowed")
		}
	}
}

func validateExtra(root map[string]any, violations *[]Violation) {
	ensureObjectField(root, "extra", violations)
}

func validateCustomSection(path string, value any, violations *[]Violation) {
	obj, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return
	}
	*violations = append(*violations, validateValueLimits(obj, path, customLimits)...)
	validateCustomSectionCoreKeys(path, obj, violations)
}

// Only the top-level keys of a custom_* section are blocked from shadowing core/promoted
// field names; nested keys (e.g. metadata.components) are intentionally permitted. See
// TestNormalizeEntity_CustomSectionAllowsNestedCoreLikeKeys.
func validateCustomSectionCoreKeys(path string, obj map[string]any, violations *[]Violation) {
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := promotedFields[key]; ok {
			appendViolation(violations, joinPath(path, key), "CORE_FIELD", "must not duplicate a promoted field or core section")
			continue
		}
		if _, ok := coreCustomSectionKeys[key]; ok {
			appendViolation(violations, joinPath(path, key), "CORE_FIELD", "must not duplicate a promoted field or core section")
		}
	}
}

func requireString(obj map[string]any, key, path string, violations *[]Violation) string {
	value, ok := obj[key]
	if !ok {
		appendViolation(violations, path, "REQUIRED", "is required")
		return ""
	}
	text, ok := value.(string)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be a string")
		return ""
	}
	if strings.TrimSpace(text) == "" {
		appendViolation(violations, path, "INVALID_VALUE", "must be a non-empty string")
		return ""
	}
	return text
}

func optionalString(obj map[string]any, key, path string, violations *[]Violation) string {
	value, ok := obj[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be a string")
		return ""
	}
	return text
}

func optionalNonEmptyString(obj map[string]any, key, path string, violations *[]Violation) string {
	value, ok := obj[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be a string")
		return ""
	}
	if strings.TrimSpace(text) == "" {
		appendViolation(violations, path, "INVALID_VALUE", "must be a non-empty string")
		return ""
	}
	return text
}

func optionalObject(obj map[string]any, key, path string, violations *[]Violation) map[string]any {
	value, ok := obj[key]
	if !ok {
		return nil
	}
	child, ok := value.(map[string]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an object")
		return nil
	}
	return child
}

func optionalArray(obj map[string]any, key, path string, violations *[]Violation) []any {
	value, ok := obj[key]
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be an array")
		return nil
	}
	return items
}

func optionalNumber(obj map[string]any, key, path string, violations *[]Violation) (float64, bool) {
	value, ok := obj[key]
	if !ok {
		return 0, false
	}
	number, ok := value.(float64)
	if !ok {
		appendViolation(violations, path, "INVALID_TYPE", "must be a number")
		return 0, false
	}
	return number, true
}

func optionalInteger(obj map[string]any, key, path string, violations *[]Violation) (int64, bool) {
	number, ok := optionalNumber(obj, key, path, violations)
	if !ok {
		return 0, false
	}
	if math.Trunc(number) != number {
		appendViolation(violations, path, "INVALID_TYPE", "must be an integer")
		return 0, false
	}
	return int64(number), true
}

func optionalRFC3339(obj map[string]any, key, path string, violations *[]Violation) {
	text := optionalString(obj, key, path, violations)
	if text == "" {
		return
	}
	if _, err := time.Parse(time.RFC3339, text); err != nil {
		appendViolation(violations, path, "INVALID_VALUE", "must be an RFC 3339 timestamp")
	}
}

func requireRFC3339(obj map[string]any, key, path string, violations *[]Violation) {
	text := requireString(obj, key, path, violations)
	if text == "" {
		return
	}
	if _, err := time.Parse(time.RFC3339, text); err != nil {
		appendViolation(violations, path, "INVALID_VALUE", "must be an RFC 3339 timestamp")
	}
}

func validateOnlyAllowedKeys(obj map[string]any, path string, allowed []string, violations *[]Violation) {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := allowedSet[key]; !ok {
			appendViolation(violations, joinPath(path, key), "UNKNOWN_FIELD", "is not allowed")
		}
	}
}
