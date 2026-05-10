package service

import (
	"context"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/protocolvalidation"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

type EntityFunctions struct {
	pgStore   ports.EntityStore
	log       *logging.Logger
	validator protocolvalidation.JSONValidator
}

func (f EntityFunctions) protocolValidator() protocolvalidation.JSONValidator {
	if f.validator != nil {
		return f.validator
	}
	return protocolvalidation.NewRunner()
}

func (f EntityFunctions) CreateEntity(ctx context.Context, entity *model.Entity) error {
	if err := validateEntityModel(entity); err != nil {
		return err
	}
	if err := f.protocolValidator().NormalizeEntity(ctx, entity, protocolvalidation.OperationCreate); err != nil {
		return err
	}
	now := time.Now().UTC()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	if entity.UpdatedAt.IsZero() {
		entity.UpdatedAt = now
	}
	f.log.InfoContext(ctx, "entity", "creating entity", logging.String("entity_id", entity.EntityID), logging.String("entity_type", string(entity.Type)))
	return f.pgStore.CreateEntity(ctx, entity)
}

func (f EntityFunctions) GetEntity(ctx context.Context, entityID string) (*model.Entity, error) {
	if entityID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	return f.pgStore.GetEntity(ctx, entityID)
}

func (f EntityFunctions) ListEntities(ctx context.Context, filters ...ports.EntityFilter) ([]model.Entity, error) {
	return f.pgStore.ListEntities(ctx, filters...)
}

func (f EntityFunctions) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	if err := validateEntityModel(entity); err != nil {
		return err
	}
	if err := f.protocolValidator().NormalizeEntity(ctx, entity, protocolvalidation.OperationUpdate); err != nil {
		return err
	}
	entity.UpdatedAt = time.Now().UTC()
	f.log.InfoContext(ctx, "entity", "updating entity", logging.String("entity_id", entity.EntityID), logging.String("entity_type", string(entity.Type)))
	return f.pgStore.UpdateEntity(ctx, entity)
}

func (f EntityFunctions) DeleteEntity(ctx context.Context, entityID string) error {
	if entityID == "" {
		return model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	f.log.InfoContext(ctx, "entity", "deleting entity", logging.String("entity_id", entityID))
	return f.pgStore.DeleteEntity(ctx, entityID)
}

func (f EntityFunctions) UpsertEntity(ctx context.Context, entity *model.Entity) error {
	if err := validateEntityModel(entity); err != nil {
		return err
	}
	if err := f.protocolValidator().NormalizeEntity(ctx, entity, protocolvalidation.OperationUpsert); err != nil {
		return err
	}
	now := time.Now().UTC()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	entity.UpdatedAt = now
	f.log.InfoContext(ctx, "entity", "upserting entity", logging.String("entity_id", entity.EntityID), logging.String("entity_type", string(entity.Type)))
	return f.pgStore.UpsertEntity(ctx, entity)
}
