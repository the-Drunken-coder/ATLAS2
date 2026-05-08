package blobvalidation

var taskAllowedTopLevel = map[string]struct{}{"description": {}, "created_by": {}, "components": {}, "extra": {}}

func validateTask(root map[string]any, _ Operation, violations *[]Violation) {
	validateAllowedTopLevelKeys(root, taskAllowedTopLevel, violations)
	validateExtra(root, violations)
	validateTopLevelCustomSections(root, violations)
	optionalString(root, "description", "json.description", violations)
	optionalString(root, "created_by", "json.created_by", violations)

	components := requireObjectField(root, "components", "json.components", violations)
	if components == nil {
		return
	}
	validateOnlyAllowedKeys(components, "json.components", []string{"command", "parameters", "progress", "result", "error"}, violations)

	command := requireObjectField(components, "command", "json.components.command", violations)
	if command != nil {
		validateOnlyAllowedKeys(command, "json.components.command", []string{"type"}, violations)
		requireString(command, "type", "json.components.command.type", violations)
	}
	parameters := requireObjectField(components, "parameters", "json.components.parameters", violations)
	if parameters != nil {
		_ = parameters
	}
	if progress := optionalObject(components, "progress", "json.components.progress", violations); progress != nil {
		_ = progress
	}
	if result := optionalObject(components, "result", "json.components.result", violations); result != nil {
		_ = result
	}
	if errObj := optionalObject(components, "error", "json.components.error", violations); errObj != nil {
		_ = errObj
	}
}
