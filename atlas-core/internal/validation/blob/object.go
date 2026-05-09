package blobvalidation

import (
	"sort"

	"github.com/anomalyco/atlas-core/internal/model"
)

// pinnedCommandCatalogObjectID matches tasks' default catalog reference; catalog
// JSON carries a keyed command map at top level (see vertical-slice-2 SPEC).
const pinnedCommandCatalogObjectID = "command_catalog"

func validateObject(root map[string]any, objectType model.ObjectType, objectID string, op Operation, violations *[]Violation) {
	_ = op // operation context reserved for future patch-style writes
	if _, ok := root["manifest"]; ok {
		appendViolation(violations, "json.manifest", "RESERVED_FIELD", "is reserved")
	}
	if _, ok := root["manifest_version"]; ok {
		appendViolation(violations, "json.manifest_version", "RESERVED_FIELD", "is reserved")
	}
	allowed := map[string]struct{}{"extra": {}, "manifest": {}, "manifest_version": {}}
	switch objectType {
	case model.ObjectTypeLog:
		allowed["log_type"] = struct{}{}
		allowed["started_at"] = struct{}{}
		allowed["ended_at"] = struct{}{}
	case model.ObjectTypePhoto:
		allowed["content_type"] = struct{}{}
		allowed["captured_at"] = struct{}{}
		allowed["width_px"] = struct{}{}
		allowed["height_px"] = struct{}{}
	case model.ObjectTypeDocument:
		allowed["content_type"] = struct{}{}
		if objectID == pinnedCommandCatalogObjectID {
			allowed["commands"] = struct{}{}
		}
	}
	validateAllowedTopLevelKeys(root, allowed, violations)
	validateExtra(root, violations)
	validateTopLevelCustomSections(root, violations)

	switch objectType {
	case model.ObjectTypeLog:
		optionalString(root, "log_type", "json.log_type", violations)
		optionalRFC3339(root, "started_at", "json.started_at", violations)
		optionalRFC3339(root, "ended_at", "json.ended_at", violations)
	case model.ObjectTypePhoto:
		optionalString(root, "content_type", "json.content_type", violations)
		optionalRFC3339(root, "captured_at", "json.captured_at", violations)
		if width, ok := optionalNumber(root, "width_px", "json.width_px", violations); ok && !isPositiveInteger(width) {
			appendViolation(violations, "json.width_px", "OUT_OF_RANGE", "must be a positive integer")
		}
		if height, ok := optionalNumber(root, "height_px", "json.height_px", violations); ok && !isPositiveInteger(height) {
			appendViolation(violations, "json.height_px", "OUT_OF_RANGE", "must be a positive integer")
		}
	case model.ObjectTypeDocument:
		optionalString(root, "content_type", "json.content_type", violations)
		if objectID == pinnedCommandCatalogObjectID {
			if _, ok := root["commands"]; ok {
				commands := optionalObject(root, "commands", "json.commands", violations)
				if commands != nil {
					commandNames := make([]string, 0, len(commands))
					for cmdName := range commands {
						commandNames = append(commandNames, cmdName)
					}
					sort.Strings(commandNames)
					for _, cmdName := range commandNames {
						cmdValue := commands[cmdName]
						cmdPath := joinPath("json.commands", cmdName)
						cmdObj, ok := cmdValue.(map[string]any)
						if !ok {
							appendViolation(violations, cmdPath, "INVALID_TYPE", "must be an object")
							continue
						}
						paramsSchema, ok := cmdObj["parameters_schema"]
						if !ok {
							appendViolation(violations, joinPath(cmdPath, "parameters_schema"), "REQUIRED", "is required")
							continue
						}
						if _, ok := paramsSchema.(map[string]any); !ok {
							appendViolation(violations, joinPath(cmdPath, "parameters_schema"), "INVALID_TYPE", "must be an object")
						}
					}
				}
			}
		}
	}
}
