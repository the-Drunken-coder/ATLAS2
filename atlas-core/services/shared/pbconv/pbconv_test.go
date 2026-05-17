package pbconv

import (
	"errors"
	"strings"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
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

func TestEntityFiltersFromProtoRejectsInvalidOptionalTimestamp(t *testing.T) {
	_, err := EntityFiltersFromProto(&sharedv1.EntityFilter{
		UpdatedAfter: &timestamppb.Timestamp{Seconds: 253402300800},
	})
	var fieldErr *model.FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected field error, got %v", err)
	}
	if fieldErr.Field != "updated_after" {
		t.Fatalf("expected updated_after field, got %q", fieldErr.Field)
	}
}

func TestEntityFiltersFromProtoIncludesValidOptionalTimestamp(t *testing.T) {
	ts := timestamppb.Now()
	filters, err := EntityFiltersFromProto(&sharedv1.EntityFilter{UpdatedAfter: ts})
	if err != nil {
		t.Fatalf("entity filters: %v", err)
	}
	query := &store.EntityFilterState{}
	for _, filter := range filters {
		filter(query)
	}
	if query.UpdatedAfter == nil || !query.UpdatedAfter.Equal(ts.AsTime().UTC()) {
		t.Fatalf("expected updated_after %v, got %v", ts.AsTime().UTC(), query.UpdatedAfter)
	}
}
