package blob

import (
	"fmt"
	"math"
	"reflect"
)

func ValidateCommandSchema(schema map[string]any, value any) error {
	violations := make([]Violation, 0)
	validateSchemaNode(schema, value, "json.components.parameters", &violations)
	return newValidationError(violations)
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
		for key, child := range obj {
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
