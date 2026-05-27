package enginenames

import "github.com/anomalyco/atlas-core/services/shared/fusionenginenames"

// All returns registered fusion engine names (registry and eval scenario validation).
func All() []string {
	return fusionenginenames.All()
}
