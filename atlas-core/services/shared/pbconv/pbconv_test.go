package pbconv

import (
	"strings"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEntityFromProtoRejectsMissingTimestamp(t *testing.T) {
	_, err := EntityFromProto(&sharedv1.Entity{
		EntityId:  "ent_001",
		Type:      "asset",
		Json:      []byte(`{}`),
		Version:   1,
		UpdatedAt: timestamppb.Now(),
	})
	if err == nil || err.Error() != "entity.created_at is required" {
		t.Fatalf("expected missing created_at error, got %v", err)
	}
}

func TestManifestFromProtoRejectsInvalidTimestamp(t *testing.T) {
	_, err := ManifestFromProto(&sharedv1.ObjectManifest{
		Version: "1",
		Files: map[string]*sharedv1.ObjectFileInfo{
			"bad.txt": {
				Size:      1,
				UpdatedAt: &timestamppb.Timestamp{Seconds: 253402300800},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `manifest.files["bad.txt"].updated_at is invalid`) {
		t.Fatalf("expected invalid updated_at error, got %v", err)
	}
}
