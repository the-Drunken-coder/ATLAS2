package atlasio

import (
	"context"
	"fmt"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Source struct {
	Client functionsv1.AtlasFunctionsServiceClient
}

func (s Source) Fetch(ctx context.Context, query core.ObservationQuery) (core.ObservationBatch, error) {
	if s.Client == nil {
		return core.ObservationBatch{}, fmt.Errorf("atlas functions client is required")
	}
	filter := &sharedv1.ObservationFilter{}
	if !query.Checkpoint.UpdatedAt.IsZero() {
		filter.UpdatedAfter = timestamppb.New(query.Checkpoint.UpdatedAt.UTC())
	}
	resp, err := s.Client.ListObservations(ctx, &sharedv1.ListObservationsRequest{
		Filter:   filter,
		PageSize: query.PageSize,
	})
	if err != nil {
		return core.ObservationBatch{}, err
	}
	var observations []core.ObservationInput
	nextCheckpoint := query.Checkpoint
	for _, protoObs := range resp.GetObservations() {
		obs, err := pbconv.ObservationFromProto(protoObs)
		if err != nil {
			return core.ObservationBatch{}, err
		}
		if obs.LatestTelemetryAt == nil {
			updatedAt := obs.UpdatedAt.UTC()
			if updatedAt.After(nextCheckpoint.UpdatedAt) ||
				(updatedAt.Equal(nextCheckpoint.UpdatedAt) && obs.ObservationID > nextCheckpoint.ObservationID) ||
				(updatedAt.Equal(nextCheckpoint.UpdatedAt) && obs.ObservationID == nextCheckpoint.ObservationID && obs.Version > nextCheckpoint.Version) {
				nextCheckpoint.UpdatedAt = updatedAt
				nextCheckpoint.ObservationID = obs.ObservationID
				nextCheckpoint.Version = obs.Version
			}
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
	return core.NewObservationBatch(observations, nextCheckpoint), nil
}
