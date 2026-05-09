package service

import (
	"context"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
	"github.com/anomalyco/atlas-core/internal/validation/blob"
)

type EntityFunctions struct {
	pgStore ports.EntityStore
	log     *logging.Logger
}

func (f EntityFunctions) CreateEntity(ctx context.Context, entity *model.Entity) error {
	if err := validateEntityModel(entity); err != nil {
		return err
	}
	if err := blob.NormalizeEntity(entity, blob.OperationCreate); err != nil {
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
	if err := blob.NormalizeEntity(entity, blob.OperationUpdate); err != nil {
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
	if err := blob.NormalizeEntity(entity, blob.OperationUpsert); err != nil {
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
