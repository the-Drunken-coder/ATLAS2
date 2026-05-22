package objectpath

import "testing"

func TestValidateObjectID(t *testing.T) {
	tests := []struct {
		objectID string
		valid    bool
	}{
		{"obj_test", true},
		{"obj-test", true},
		{"", false},
		{".", false},
		{"..", false},
		{ManifestFilename, false},
		{"obj.with.dot", false},
		{"obj/test", false},
	}

	for _, tt := range tests {
		err := ValidateObjectID(tt.objectID)
		if tt.valid && err != nil {
			t.Errorf("expected valid object_id %q, got error: %v", tt.objectID, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("expected invalid object_id %q, got no error", tt.objectID)
		}
	}
}

func TestValidateDeletableFolderNameAllowsLegacyNames(t *testing.T) {
	if err := ValidateDeletableFolderName("backup.2025-05-04"); err != nil {
		t.Fatalf("expected deletable legacy folder name, got %v", err)
	}
	if err := ValidateObjectID("backup.2025-05-04"); err == nil {
		t.Fatal("expected strict object ID validation to reject dotted legacy name")
	}
}
