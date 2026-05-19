package pbconv

import (
	"errors"
	"strings"
	"testing"
	"time"

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

func TestObservationProtoRoundTripIncludesQueryableFields(t *testing.T) {
	observedAt := time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC)
	targetEntityID := "track_001"
	obs := &model.Observation{
		ObservationID:  "obs_001",
		SourceAssetID:  "asset_001",
		TargetEntityID: &targetEntityID,
		ObservedAt:     &observedAt,
		JSON:           []byte(`{"state":"active"}`),
		Version:        3,
		CreatedAt:      observedAt.Add(-time.Minute),
		UpdatedAt:      observedAt,
	}

	converted, err := ObservationFromProto(ObservationToProto(obs))
	if err != nil {
		t.Fatalf("ObservationFromProto: %v", err)
	}
	if converted.TargetEntityID == nil || *converted.TargetEntityID != targetEntityID {
		t.Fatalf("expected target_entity_id %q, got %v", targetEntityID, converted.TargetEntityID)
	}
	if converted.ObservedAt == nil || !converted.ObservedAt.Equal(observedAt) {
		t.Fatalf("expected observed_at %v, got %v", observedAt, converted.ObservedAt)
	}
}

func TestObservationFiltersFromProtoIncludesQueryableFields(t *testing.T) {
	observedAtFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	observedAtTo := observedAtFrom.Add(time.Hour)
	filters, err := ObservationFiltersFromProto(&sharedv1.ObservationFilter{
		SourceAssetId:  stringPtr("asset_001"),
		TargetEntityId: stringPtr("track_001"),
		ObservedAtFrom: timestamppb.New(observedAtFrom),
		ObservedAtTo:   timestamppb.New(observedAtTo),
	})
	if err != nil {
		t.Fatalf("ObservationFiltersFromProto: %v", err)
	}
	query := &store.ObservationFilterState{}
	for _, filter := range filters {
		filter(query)
	}
	if query.SourceAssetID == nil || *query.SourceAssetID != "asset_001" {
		t.Fatalf("expected source asset filter, got %v", query.SourceAssetID)
	}
	if query.TargetEntityID == nil || *query.TargetEntityID != "track_001" {
		t.Fatalf("expected target entity filter, got %v", query.TargetEntityID)
	}
	if query.ObservedAtFrom == nil || !query.ObservedAtFrom.Equal(observedAtFrom) {
		t.Fatalf("expected observed_at_from %v, got %v", observedAtFrom, query.ObservedAtFrom)
	}
	if query.ObservedAtTo == nil || !query.ObservedAtTo.Equal(observedAtTo) {
		t.Fatalf("expected observed_at_to %v, got %v", observedAtTo, query.ObservedAtTo)
	}
}
