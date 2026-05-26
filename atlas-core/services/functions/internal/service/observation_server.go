package service

import (
	"context"

	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/store"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *Server) CreateObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := pbconv.ObservationFromProto(defaultObservationRequestTimestamps(req.GetObservation()))
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Observation.CreateObservation(ctx, observation); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
func (s *Server) GetObservation(ctx context.Context, req *sharedv1.GetObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := s.funcs.Observation.GetObservation(ctx, req.GetObservationId())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
func (s *Server) ListObservations(ctx context.Context, req *sharedv1.ListObservationsRequest) (*sharedv1.ListObservationsResponse, error) {
	filters, err := pbconv.ObservationFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	result, err := s.funcs.Observation.ListObservations(ctx, store.ObservationListParams{
		Filters:        filters,
		PageSize:       req.GetPageSize(),
		PageToken:      req.GetPageToken(),
		StrictSnapshot: req.GetStrictSnapshot(),
	})
	if err != nil {
		return nil, s.status(ctx, err)
	}
	resp := &sharedv1.ListObservationsResponse{NextPageToken: result.NextPageToken}
	for i := range result.Observations {
		resp.Observations = append(resp.Observations, pbconv.ObservationToProto(&result.Observations[i]))
	}
	return resp, nil
}
func (s *Server) UpdateObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := pbconv.ObservationFromProto(req.GetObservation())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Observation.UpdateObservation(ctx, observation); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
func (s *Server) DeleteObservation(ctx context.Context, req *sharedv1.DeleteObservationRequest) (*emptypb.Empty, error) {
	if err := s.funcs.Observation.DeleteObservation(ctx, req.GetObservationId()); err != nil {
		return nil, s.status(ctx, err)
	}
	return &emptypb.Empty{}, nil
}
func (s *Server) UpsertObservation(ctx context.Context, req *sharedv1.ObservationRequest) (*sharedv1.ObservationResponse, error) {
	observation, err := pbconv.ObservationFromProto(defaultObservationRequestTimestamps(req.GetObservation()))
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Observation.UpsertObservation(ctx, observation); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}

func (s *Server) IngestObservationTelemetry(ctx context.Context, req *sharedv1.IngestObservationTelemetryRequest) (*sharedv1.ObservationResponse, error) {
	if req == nil {
		return nil, s.status(ctx, model.NewFieldError("INVALID_INPUT", "request is required", "request"))
	}
	ingest := functionpkg.ObservationTelemetryIngest{
		ObservationID: req.GetObservationId(),
		SourceAssetID: req.GetSourceAssetId(),
		TelemetryJSON: append([]byte(nil), req.GetTelemetry()...),
	}
	if req.TargetEntityId != nil {
		targetEntityID := req.GetTargetEntityId()
		ingest.TargetEntityID = &targetEntityID
	}
	if startedAtProto := req.GetStartedAt(); startedAtProto != nil {
		startedAt, err := pbconv.TimestampFromProto(startedAtProto, "started_at")
		if err != nil {
			return nil, s.status(ctx, err)
		}
		ingest.StartedAt = startedAt
	}
	if endedAtProto := req.GetEndedAt(); endedAtProto != nil {
		endedAt, err := pbconv.TimestampFromProto(endedAtProto, "ended_at")
		if err != nil {
			return nil, s.status(ctx, err)
		}
		ingest.EndedAt = &endedAt
	}
	if len(req.GetIdentity()) > 0 {
		ingest.IdentityJSON = append([]byte(nil), req.GetIdentity()...)
	}
	observation, err := s.funcs.Observation.IngestObservationTelemetry(ctx, ingest)
	if err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObservationResponse{Observation: pbconv.ObservationToProto(observation)}, nil
}
