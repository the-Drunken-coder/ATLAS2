package blob

import (
	"fmt"
	"math"
	"reflect"
	"sort"
)

func ValidateCommandSchema(schema map[string]any, value any) error {
	violations := make([]Violation, 0)
	validateSchemaNode(schema, value, "json.components.parameters", &violations)
	return newValidationError(violations)
}

func validateCommandSchemaDefinition(schema map[string]any, path string, violations *[]Violation) {
	typeName, _ := schema["type"].(string)
	if typeName == "" {
		appendViolation(violations, path, "INVALID_SCHEMA", "schema type is required")
		return
	}
	switch typeName {
	case "object":
		if propertiesValue, ok := schema["properties"]; ok {
			properties, ok := propertiesValue.(map[string]any)
			if !ok {
				appendViolation(violations, joinPath(path, "properties"), "INVALID_SCHEMA", "schema properties must be an object")
				return
			}
			keys := make([]string, 0, len(properties))
			for key := range properties {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				childSchema, ok := properties[key].(map[string]any)
				if !ok {
					appendViolation(violations, joinPath(joinPath(path, "properties"), key), "INVALID_SCHEMA", "schema property must be an object")
					continue
				}
				validateCommandSchemaDefinition(childSchema, joinPath(joinPath(path, "properties"), key), violations)
			}
		}
		if requiredValue, ok := schema["required"]; ok {
			required, ok := requiredValue.([]any)
			if !ok {
				appendViolation(violations, joinPath(path, "required"), "INVALID_SCHEMA", "required must be an array")
				return
			}
			for idx, item := range required {
				if _, ok := item.(string); !ok {
					appendViolation(violations, fmt.Sprintf("%s[%d]", joinPath(path, "required"), idx), "INVALID_SCHEMA", "required entries must be strings")
				}
			}
		}
		if additional, ok := schema["additionalProperties"]; ok {
			if _, ok := additional.(bool); !ok {
				appendViolation(violations, joinPath(path, "additionalProperties"), "INVALID_SCHEMA", "additionalProperties must be a boolean")
			}
		}
	case "array":
		if itemSchemaValue, ok := schema["items"]; ok {
			itemSchema, ok := itemSchemaValue.(map[string]any)
			if !ok {
				appendViolation(violations, joinPath(path, "items"), "INVALID_SCHEMA", "items schema must be an object")
				return
			}
			validateCommandSchemaDefinition(itemSchema, joinPath(path, "items"), violations)
		}
	case "string", "number", "integer", "boolean":
	default:
		appendViolation(violations, path, "INVALID_SCHEMA", "schema type is not supported")
	}
}

func validateSchemaNode(schema map[string]any, value any, path string, violations *[]Violation) {
	typeName, _ := schema["type"].(string)
	if typeName == "" {
		appendViolation(violations, path, "INVALID_SCHEMA", "schema type is required")
		return
	}
	switch typeName {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			appendViolation(violations, path, "INVALID_TYPE", "must be an object")
			return
		}
		properties, _ := schema["properties"].(map[string]any)
		required, _ := schema["required"].([]any)
		for _, item := range required {
			if key, ok := item.(string); ok {
				if _, exists := obj[key]; !exists {
					appendViolation(violations, joinPath(path, key), "REQUIRED", "is required")
				}
			}
		}
		additionalProperties := true
		if allowed, ok := schema["additionalProperties"].(bool); ok {
			additionalProperties = allowed
		}
		keys := make([]string, 0, len(obj))
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := obj[key]
			childSchemaValue, ok := properties[key]
			if !ok {
				if !additionalProperties {
					appendViolation(violations, joinPath(path, key), "UNKNOWN_FIELD", "is not allowed")
				}
				continue
			}
			childSchema, ok := childSchemaValue.(map[string]any)
			if !ok {
				appendViolation(violations, joinPath(path, key), "INVALID_SCHEMA", "schema property must be an object")
				continue
			}
			validateSchemaNode(childSchema, child, joinPath(path, key), violations)
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			appendViolation(violations, path, "INVALID_TYPE", "must be an array")
			return
		}
		if itemSchemaValue, ok := schema["items"]; ok {
			itemSchema, ok := itemSchemaValue.(map[string]any)
			if !ok {
				appendViolation(violations, path, "INVALID_SCHEMA", "items schema must be an object")
				return
			}
			for idx, item := range items {
				validateSchemaNode(itemSchema, item, fmt.Sprintf("%s[%d]", path, idx), violations)
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			appendViolation(violations, path, "INVALID_TYPE", "must be a string")
			return
		}
	case "number":
		if _, ok := value.(float64); !ok {
			appendViolation(violations, path, "INVALID_TYPE", "must be a number")
			return
		}
	case "integer":
		n, ok := value.(float64)
		if !ok {
			appendViolation(violations, path, "INVALID_TYPE", "must be an integer")
			return
		}
		if n != math.Trunc(n) {
			appendViolation(violations, path, "INVALID_VALUE", "must be an integer")
			return
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			appendViolation(violations, path, "INVALID_TYPE", "must be a boolean")
			return
		}
	default:
		appendViolation(violations, path, "INVALID_SCHEMA", "schema type is not supported")
		return
	}
	if enumValues, ok := schema["enum"].([]any); ok {
		for _, candidate := range enumValues {
			if reflect.DeepEqual(candidate, value) {
				return
			}
		}
		appendViolation(violations, path, "INVALID_VALUE", "must match an allowed value")
	}
}
