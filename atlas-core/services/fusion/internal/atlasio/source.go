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
	pageToken := ""
	for {
		resp, err := s.Client.ListObservations(ctx, &sharedv1.ListObservationsRequest{
			Filter:    filter,
			PageSize:  query.PageSize,
			PageToken: pageToken,
		})
		if err != nil {
			return core.ObservationBatch{}, err
		}
		for _, protoObs := range resp.GetObservations() {
			obs, err := pbconv.ObservationFromProto(protoObs)
			if err != nil {
				return core.ObservationBatch{}, err
			}
			if obs.LatestTelemetryAt == nil {
				continue
			}
			input := core.ObservationInput{
				ObservationID:     obs.ObservationID,
				SourceAssetID:     obs.SourceAssetID,
				TargetEntityID:    obs.TargetEntityID,
				LatestTelemetryAt: obs.LatestTelemetryAt.UTC(),
				Version:           obs.Version,
				UpdatedAt:         obs.UpdatedAt.UTC(),
				JSON:              append([]byte(nil), obs.JSON...),
			}
			if core.AfterCheckpoint(input, query.Checkpoint) {
				observations = append(observations, input)
			}
		}
		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			break
		}
	}
	return core.NewObservationBatch(observations, query.Checkpoint), nil
}
