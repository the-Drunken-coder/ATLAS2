package manifestvalidation

import (
	"errors"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/model"
)

func TestValidateObjectManifest_UsesDeterministicFileOrder(t *testing.T) {
	manifest := &model.ObjectManifest{
		Files: map[string]model.ObjectFileInfo{
			"manifest.json": {Size: 1, UpdatedAt: time.Now().UTC()},
			"":              {Size: -1},
		},
	}

	err := ValidateObjectManifest(manifest)
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected field error, got %v", err)
	}
	if fieldErr.Field != "manifest.files." {
		t.Fatalf("expected blank filename to fail first, got %s", fieldErr.Field)
	}
}
