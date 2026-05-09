package manifest

import (
	"sort"

	"github.com/anomalyco/atlas-core/internal/core/model"
)

func ValidateObjectManifest(manifest *model.ObjectManifest) error {
	if manifest == nil {
		return model.NewFieldError("INVALID_INPUT", "manifest is required", "manifest")
	}
	names := make([]string, 0, len(manifest.Files))
	for name := range manifest.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		info := manifest.Files[name]
		field := "manifest.files." + name
		if err := model.ValidateObjectFilename(name); err != nil {
			return model.NewFieldError("INVALID_INPUT", err.Error(), field)
		}
		if info.Size < 0 {
			return model.NewFieldError("INVALID_INPUT", "manifest file size must be non-negative", field)
		}
		if info.UpdatedAt.IsZero() {
			return model.NewFieldError("INVALID_INPUT", "manifest file updated_at is required", field)
		}
	}
	return nil
}
