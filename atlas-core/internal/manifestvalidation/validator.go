package manifestvalidation

import (
	"strings"

	"github.com/anomalyco/atlas-core/internal/model"
)

func ValidateObjectManifest(manifest *model.ObjectManifest) error {
	if manifest == nil {
		return model.NewFieldError("INVALID_INPUT", "manifest is required", "manifest")
	}
	for name, info := range manifest.Files {
		field := "manifest.files." + name
		if strings.TrimSpace(name) == "" {
			return model.NewFieldError("INVALID_INPUT", "manifest file names must be non-empty", field)
		}
		if name == "manifest.json" {
			return model.NewFieldError("INVALID_INPUT", "manifest.json is reserved", field)
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
