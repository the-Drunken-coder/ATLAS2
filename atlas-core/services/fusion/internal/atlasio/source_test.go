package atlasio

import (
	"context"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"google.golang.org/grpc"
)

type fakeListObservationsClient struct {
	listFn func(context.Context, *sharedv1.ListObservationsRequest) (*sharedv1.ListObservationsResponse, error)
}

func (c *fakeListObservationsClient) ListObservations(ctx context.Context, req *sharedv1.ListObservationsRequest, _ ...grpc.CallOption) (*sharedv1.ListObservationsResponse, error) {
	return c.listFn(ctx, req)
}

func TestSourceFetchPaginatesAndOmitsTelemetryLessRowsFromBatch(t *testing.T) {
	t4 := time.Date(2026, 1, 1, 4, 0, 0, 0, time.UTC)
	t5 := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)
	t6 := time.Date(2026, 1, 1, 6, 0, 0, 0, time.UTC)
	telemetryAt := time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC)

	page1 := []*sharedv1.Observation{
		protoObservation("obs_identity", t5, nil, 1),
		protoObservation("obs_fuse", t6, &telemetryAt, 1),
	}
	page2 := []*sharedv1.Observation{
		protoObservation("obs_old", t4, &telemetryAt, 1),
	}

	client := &fakeListObservationsClient{
		listFn: func(_ context.Context, req *sharedv1.ListObservationsRequest) (*sharedv1.ListObservationsResponse, error) {
			switch req.GetPageToken() {
			case "":
				return &sharedv1.ListObservationsResponse{
					Observations:  page1,
					NextPageToken: "page2",
				}, nil
			case "page2":
				return &sharedv1.ListObservationsResponse{Observations: page2}, nil
			default:
				t.Fatalf("unexpected page token %q", req.GetPageToken())
				return nil, nil
			}
		},
	}

	batch, err := Source{Client: client}.Fetch(context.Background(), core.ObservationQuery{PageSize: 2})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(batch.Observations) != 2 {
		t.Fatalf("expected exactly 2 fused observations, got %d: %#v", len(batch.Observations), batch.Observations)
	}
	ids := make(map[string]bool, len(batch.Observations))
	for _, obs := range batch.Observations {
		ids[obs.ObservationID] = true
	}
	if !ids["obs_fuse"] || !ids["obs_old"] {
		t.Fatalf("expected obs_fuse and obs_old in batch, got %#v", batch.Observations)
	}
	if ids["obs_identity"] {
		t.Fatal("expected identity-only row to be skipped")
	}
	if !batch.NextCheckpoint.UpdatedAt.Equal(t6) || batch.NextCheckpoint.ObservationID != "obs_fuse" {
		t.Fatalf("expected checkpoint at latest listed row, got %+v", batch.NextCheckpoint)
	}
}

func TestSourceFetchAdvancesCheckpointPastTelemetryLessRows(t *testing.T) {
	t5 := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)
	client := &fakeListObservationsClient{
		listFn: func(_ context.Context, _ *sharedv1.ListObservationsRequest) (*sharedv1.ListObservationsResponse, error) {
			return &sharedv1.ListObservationsResponse{
				Observations: []*sharedv1.Observation{
					protoObservation("obs_identity", t5, nil, 1),
				},
			}, nil
		},
	}
	batch, err := Source{Client: client}.Fetch(context.Background(), core.ObservationQuery{PageSize: 10})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(batch.Observations) != 0 {
		t.Fatalf("expected empty batch, got %#v", batch.Observations)
	}
	if !batch.NextCheckpoint.UpdatedAt.Equal(t5) || batch.NextCheckpoint.ObservationID != "obs_identity" {
		t.Fatalf("expected checkpoint past telemetry-less row, got %+v", batch.NextCheckpoint)
	}
}

func protoObservation(id string, updatedAt time.Time, latestTelemetryAt *time.Time, version int) *sharedv1.Observation {
	obs := &model.Observation{
		ObservationID:     id,
		SourceAssetID:     "asset_001",
		StartedAt:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Version:           version,
		JSON:              []byte(`{"identity":{"kind":"asset"}}`),
		CreatedAt:         updatedAt,
		UpdatedAt:         updatedAt,
		LatestTelemetryAt: latestTelemetryAt,
	}
	return pbconv.ObservationToProto(obs)
}
