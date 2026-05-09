package manifest

import (
	"errors"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
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

func TestValidateObjectManifest_RejectsUnsafeFileNames(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name      string
		filename  string
		wantField string
	}{
		{name: "path separator", filename: "nested/file.txt", wantField: "manifest.files.nested/file.txt"},
		{name: "absolute path", filename: "/tmp/file.txt", wantField: "manifest.files./tmp/file.txt"},
		{name: "dot segment", filename: "..", wantField: "manifest.files..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateObjectManifest(&model.ObjectManifest{
				Files: map[string]model.ObjectFileInfo{
					tt.filename: {Size: 1, UpdatedAt: now},
				},
			})
			var fieldErr *model.FieldError
			if !errors.As(err, &fieldErr) {
				t.Fatalf("expected field error, got %v", err)
			}
			if fieldErr.Field != tt.wantField {
				t.Fatalf("expected %s, got %s", tt.wantField, fieldErr.Field)
			}
		})
	}
}
