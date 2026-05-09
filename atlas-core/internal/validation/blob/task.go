package blob

var taskAllowedTopLevel = map[string]struct{}{"description": {}, "created_by": {}, "components": {}, "extra": {}}

func validateTask(root map[string]any, op Operation, violations *[]Violation) {
	_ = op // operation context reserved for future patch-style writes
	validateAllowedTopLevelKeys(root, taskAllowedTopLevel, violations)
	validateExtra(root, violations)
	validateTopLevelCustomSections(root, violations)
	optionalString(root, "description", "json.description", violations)
	optionalString(root, "created_by", "json.created_by", violations)

	components := requireObjectFieldOrEmpty(root, "components", "json.components", violations)
	if components == nil {
		return
	}
	validateOnlyAllowedKeys(components, "json.components", []string{"command", "parameters", "progress", "result", "error"}, violations)

	command := requireObjectFieldOrEmpty(components, "command", "json.components.command", violations)
	if command != nil {
		validateOnlyAllowedKeys(command, "json.components.command", []string{"type"}, violations)
		requireString(command, "type", "json.components.command.type", violations)
	}
	requireObjectFieldOrEmpty(components, "parameters", "json.components.parameters", violations)
	optionalObject(components, "progress", "json.components.progress", violations)
	optionalObject(components, "result", "json.components.result", violations)
	optionalObject(components, "error", "json.components.error", violations)
}
