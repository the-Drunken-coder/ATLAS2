package blob

import "sort"

func validateTopLevelCustomSections(root map[string]any, violations *[]Violation) {
	keys := make([]string, 0, len(root))
	for key := range root {
		if isCustomKey(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		validateCustomSection(joinPath("json", key), root[key], violations)
	}
}
