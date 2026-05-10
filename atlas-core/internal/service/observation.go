package service

import (
	"context"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/protocolvalidation"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

type ObservationFunctions struct {
	pgStore   ports.ObservationStore
	log       *logging.Logger
	validator protocolvalidation.JSONValidator
}

func (f ObservationFunctions) protocolValidator() protocolvalidation.JSONValidator {
	if f.validator != nil {
		return f.validator
	}
	return protocolvalidation.NewRunner()
}

func (f ObservationFunctions) CreateObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs); err != nil {
		return err
	}
	if err := f.protocolValidator().NormalizeObservation(obs, protocolvalidation.OperationCreate); err != nil {
		return err
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	if obs.UpdatedAt.IsZero() {
		obs.UpdatedAt = now
	}
	f.log.InfoContext(ctx, "observation", "creating observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	return f.pgStore.CreateObservation(ctx, obs)
}

func (f ObservationFunctions) GetObservation(ctx context.Context, observationID string) (*model.Observation, error) {
	if observationID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	return f.pgStore.GetObservation(ctx, observationID)
}

func (f ObservationFunctions) ListObservations(ctx context.Context, filters ...ports.ObservationFilter) ([]model.Observation, error) {
	return f.pgStore.ListObservations(ctx, filters...)
}

func (f ObservationFunctions) UpdateObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs); err != nil {
		return err
	}
	if err := f.protocolValidator().NormalizeObservation(obs, protocolvalidation.OperationUpdate); err != nil {
		return err
	}
	obs.UpdatedAt = time.Now().UTC()
	f.log.InfoContext(ctx, "observation", "updating observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	return f.pgStore.UpdateObservation(ctx, obs)
}

func (f ObservationFunctions) DeleteObservation(ctx context.Context, observationID string) error {
	if observationID == "" {
		return model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	f.log.InfoContext(ctx, "observation", "deleting observation", logging.String("observation_id", observationID))
	return f.pgStore.DeleteObservation(ctx, observationID)
}

func (f ObservationFunctions) UpsertObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs); err != nil {
		return err
	}
	if err := f.protocolValidator().NormalizeObservation(obs, protocolvalidation.OperationUpsert); err != nil {
		return err
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	obs.UpdatedAt = now
	f.log.InfoContext(ctx, "observation", "upserting observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	return f.pgStore.UpsertObservation(ctx, obs)
}
