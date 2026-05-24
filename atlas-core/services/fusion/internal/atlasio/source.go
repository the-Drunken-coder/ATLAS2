package atlasio

import (
	"context"
	"fmt"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maxStrictSnapshotPasses = 3

type ListObservationsClient interface {
	ListObservations(ctx context.Context, in *sharedv1.ListObservationsRequest, opts ...grpc.CallOption) (*sharedv1.ListObservationsResponse, error)
}

type Source struct {
	Client ListObservationsClient
}

func (s Source) Fetch(ctx context.Context, query core.ObservationQuery) (core.ObservationBatch, error) {
	if s.Client == nil {
		return core.ObservationBatch{}, fmt.Errorf("atlas functions client is required")
	}
	filter := &sharedv1.ObservationFilter{}
	if !query.Checkpoint.UpdatedAt.IsZero() {
		filter.UpdatedAfter = timestamppb.New(query.Checkpoint.UpdatedAt.UTC())
	}

	var observations []core.ObservationInput
	nextCheckpoint := query.Checkpoint
	var prevListedIDs map[string]struct{}

	for pass := 0; pass < maxStrictSnapshotPasses; pass++ {
		listedIDs := make(map[string]struct{})
		var passObs []core.ObservationInput
		passCheckpoint := query.Checkpoint
		pageToken := ""

		for {
			resp, err := s.Client.ListObservations(ctx, &sharedv1.ListObservationsRequest{
				Filter:         filter,
				PageSize:       query.PageSize,
				PageToken:      pageToken,
				StrictSnapshot: true,
			})
			if err != nil {
				return core.ObservationBatch{}, err
			}
			for _, protoObs := range resp.GetObservations() {
				obs, err := pbconv.ObservationFromProto(protoObs)
				if err != nil {
					return core.ObservationBatch{}, err
				}
				listedIDs[obs.ObservationID] = struct{}{}
				cursor := core.ObservationInput{
					ObservationID: obs.ObservationID,
					Version:       obs.Version,
					UpdatedAt:     obs.UpdatedAt.UTC(),
				}
				passCheckpoint = core.NextCheckpoint([]core.ObservationInput{cursor}, passCheckpoint)
				if obs.LatestTelemetryAt == nil {
					continue
				}
				input := core.ObservationInput{
					ObservationID:     cursor.ObservationID,
					SourceAssetID:     obs.SourceAssetID,
					TargetEntityID:    obs.TargetEntityID,
					LatestTelemetryAt: obs.LatestTelemetryAt.UTC(),
					Version:           cursor.Version,
					UpdatedAt:         cursor.UpdatedAt,
					JSON:              append([]byte(nil), obs.JSON...),
				}
				if core.AfterCheckpoint(input, query.Checkpoint) {
					passObs = append(passObs, input)
				}
			}
			pageToken = resp.GetNextPageToken()
			if pageToken == "" {
				break
			}
		}

		observations = passObs
		nextCheckpoint = passCheckpoint
		if pass > 0 && sameIDSet(prevListedIDs, listedIDs) {
			break
		}
		prevListedIDs = listedIDs
	}

	batch := core.NewObservationBatch(observations, query.Checkpoint)
	batch.NextCheckpoint = nextCheckpoint
	return batch, nil
}

func sameIDSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if _, ok := b[id]; !ok {
			return false
		}
	}
	return true
}
