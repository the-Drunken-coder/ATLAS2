package function

import (
	"context"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type ObservationFunctions struct {
	pgStore        store.ObservationStore
	log            *logging.Logger
	protoValidator ProtocolValidator
	publisher      Publisher
}

func NewObservationFunctions(pgStore store.ObservationStore, log *logging.Logger, protoValidator ProtocolValidator, publishers ...Publisher) ObservationFunctions {
	return ObservationFunctions{pgStore: pgStore, log: log, protoValidator: protoValidator, publisher: publisherOrNop(publishers)}
}

func (f ObservationFunctions) CreateObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs); err != nil {
		return err
	}
	if obs.JSON == nil {
		obs.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	if obs.UpdatedAt.IsZero() {
		obs.UpdatedAt = now
	}
	f.log.InfoContext(ctx, "observation", "creating observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	if err := f.pgStore.CreateObservation(ctx, obs); err != nil {
		return err
	}
	publishObservation(ctx, f.publisher, "created", obs)
	return nil
}

func (f ObservationFunctions) GetObservation(ctx context.Context, observationID string) (*model.Observation, error) {
	if observationID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	return f.pgStore.GetObservation(ctx, observationID)
}

func (f ObservationFunctions) ListObservations(ctx context.Context, params store.ObservationListParams) (store.ObservationListResult, error) {
	return f.pgStore.ListObservations(ctx, params)
}

func (f ObservationFunctions) UpdateObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs); err != nil {
		return err
	}
	if obs.JSON == nil {
		obs.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	obs.UpdatedAt = time.Now().UTC()
	f.log.InfoContext(ctx, "observation", "updating observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	if err := f.pgStore.UpdateObservation(ctx, obs); err != nil {
		return err
	}
	publishObservation(ctx, f.publisher, "updated", obs)
	return nil
}

func (f ObservationFunctions) DeleteObservation(ctx context.Context, observationID string) error {
	if observationID == "" {
		return model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	f.log.InfoContext(ctx, "observation", "deleting observation", logging.String("observation_id", observationID))
	observation, err := f.pgStore.GetObservation(ctx, observationID)
	if err != nil {
		return err
	}
	if err := f.pgStore.DeleteObservation(ctx, observationID); err != nil {
		return err
	}
	publishObservation(ctx, f.publisher, "deleted", observation)
	return nil
}

func (f ObservationFunctions) UpsertObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs); err != nil {
		return err
	}
	if obs.JSON == nil {
		obs.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	obs.UpdatedAt = now
	f.log.InfoContext(ctx, "observation", "upserting observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	if err := f.pgStore.UpsertObservation(ctx, obs); err != nil {
		return err
	}
	publishObservation(ctx, f.publisher, "updated", obs)
	return nil
}
