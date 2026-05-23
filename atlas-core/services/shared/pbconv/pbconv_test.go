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
	startedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	latestTelemetryAt := time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC)
	targetEntityID := "track_001"
	obs := &model.Observation{
		ObservationID:     "obs_001",
		SourceAssetID:     "asset_001",
		TargetEntityID:    &targetEntityID,
		StartedAt:         startedAt,
		LatestTelemetryAt: &latestTelemetryAt,
		JSON:              []byte(`{"latest_telemetry":{"observed_at":"2026-01-01T00:06:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0}}}`),
		Version:           3,
		CreatedAt:         startedAt,
		UpdatedAt:         latestTelemetryAt,
	}

	converted, err := ObservationFromProto(ObservationToProto(obs))
	if err != nil {
		t.Fatalf("ObservationFromProto: %v", err)
	}
	if converted.TargetEntityID == nil || *converted.TargetEntityID != targetEntityID {
		t.Fatalf("expected target_entity_id %q, got %v", targetEntityID, converted.TargetEntityID)
	}
	if !converted.StartedAt.Equal(startedAt) {
		t.Fatalf("expected started_at %v, got %v", startedAt, converted.StartedAt)
	}
	if converted.LatestTelemetryAt == nil || !converted.LatestTelemetryAt.Equal(latestTelemetryAt) {
		t.Fatalf("expected latest_telemetry_at %v, got %v", latestTelemetryAt, converted.LatestTelemetryAt)
	}
}

func TestObservationFromProtoAllowsMissingStartedAt(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	obs, err := ObservationFromProto(&sharedv1.Observation{
		ObservationId: "obs_001",
		SourceAssetId: "asset_001",
		Json:          []byte(`{"identity":{"kind":"asset"}}`),
		Version:       2,
		CreatedAt:     timestamppb.New(createdAt),
		UpdatedAt:     timestamppb.New(updatedAt),
	})
	if err != nil {
		t.Fatalf("ObservationFromProto: %v", err)
	}
	if !obs.StartedAt.IsZero() {
		t.Fatalf("expected zero started_at when omitted, got %v", obs.StartedAt)
	}
}

func TestObservationFiltersFromProtoIncludesQueryableFields(t *testing.T) {
	startedAtFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	startedAtTo := startedAtFrom.Add(time.Hour)
	latestTelemetryAtFrom := time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC)
	filters, err := ObservationFiltersFromProto(&sharedv1.ObservationFilter{
		SourceAssetId:         stringPtr("asset_001"),
		TargetEntityId:        stringPtr("track_001"),
		StartedAtFrom:         timestamppb.New(startedAtFrom),
		StartedAtTo:           timestamppb.New(startedAtTo),
		LatestTelemetryAtFrom: timestamppb.New(latestTelemetryAtFrom),
		OpenOnly:              boolPtr(true),
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
	if query.StartedAtFrom == nil || !query.StartedAtFrom.Equal(startedAtFrom) {
		t.Fatalf("expected started_at_from %v, got %v", startedAtFrom, query.StartedAtFrom)
	}
	if query.StartedAtTo == nil || !query.StartedAtTo.Equal(startedAtTo) {
		t.Fatalf("expected started_at_to %v, got %v", startedAtTo, query.StartedAtTo)
	}
	if query.LatestTelemetryAtFrom == nil || !query.LatestTelemetryAtFrom.Equal(latestTelemetryAtFrom) {
		t.Fatalf("expected latest_telemetry_at_from %v, got %v", latestTelemetryAtFrom, query.LatestTelemetryAtFrom)
	}
	if !query.OpenOnly {
		t.Fatalf("expected open_only filter")
	}
}
