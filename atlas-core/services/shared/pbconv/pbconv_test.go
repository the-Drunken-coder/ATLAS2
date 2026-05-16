package pbconv

import (
	"errors"
	"strings"
	"testing"
	"time"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/store"
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
	if !errors.Is(err, errTimestampRequired) {
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
	if !errors.Is(err, errTimestampInvalid) || !strings.Contains(err.Error(), `manifest.files["bad.txt"].updated_at`) {
		t.Fatalf("expected invalid updated_at error, got %v", err)
	}
}

func TestEntityFiltersFromProtoHandlesInvalidOptionalTimestamp(t *testing.T) {
	filters := EntityFiltersFromProto(&sharedv1.EntityFilter{
		UpdatedAfter: &timestamppb.Timestamp{Seconds: 253402300800},
	})
	query := &store.EntityFilterState{}
	for _, filter := range filters {
		filter(query)
	}
	if query.UpdatedAfter == nil || !query.UpdatedAfter.Equal(time.Time{}) {
		t.Fatalf("expected invalid optional timestamp to fall back to zero time, got %v", query.UpdatedAfter)
	}
}
