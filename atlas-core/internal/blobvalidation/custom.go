package blobvalidation

func validateTopLevelCustomSections(root map[string]any, violations *[]Violation) {
	for key, value := range root {
		if isCustomKey(key) {
			validateCustomSection(joinPath("json", key), value, violations)
		}
	}
}
