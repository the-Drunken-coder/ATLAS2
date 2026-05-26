package reference

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
)

func TestEngineProducesProtocolValidTrackJSON(t *testing.T) {
	latestTelemetryAt := time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)
	result, err := (Engine{}).Fuse(context.Background(), core.NewObservationBatch([]core.ObservationInput{{
		ObservationID:     "obs_001",
		SourceAssetID:     "asset_adsb",
		LatestTelemetryAt: latestTelemetryAt,
		UpdatedAt:         updatedAt,
		Version:           1,
		JSON:              json.RawMessage(`{"latest_telemetry":{"observed_at":"2026-01-01T00:10:00Z","kind":"point","data":{"latitude":40.7,"longitude":-74.0,"altitude_m":1200,"uncertainty_radius_m":250}}}`),
	}}, core.Checkpoint{}))
	if err != nil {
		t.Fatalf("Fuse failed: %v", err)
	}
	if len(result.TrackUpdates) != 1 {
		t.Fatalf("expected one track update, got %+v", result.TrackUpdates)
	}
	validator, err := protocolvalidation.New()
	if err != nil {
		t.Fatalf("validator init failed: %v", err)
	}
	issues := validator.ValidateEntity(&model.Entity{
		EntityID: "track_001",
		Type:     model.EntityTypeTrack,
		JSON:     result.TrackUpdates[0].JSON,
	})
	if len(issues) > 0 {
		t.Fatalf("track JSON failed protocol validation: %+v", issues)
	}
}
