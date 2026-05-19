package function

import (
	"atlas.local/protocol"
	"context"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type Functions struct {
	Entity      EntityFunctions
	Object      ObjectFunctions
	Task        TaskFunctions
	Observation ObservationFunctions
}

type ProtocolValidator interface {
	ValidateEntity(entity *model.Entity) []protocol.ValidationIssue
	ValidateObject(obj *model.Object) []protocol.ValidationIssue
	ValidateTask(task *model.Task) []protocol.ValidationIssue
	ValidateObservation(obs *model.Observation) []protocol.ValidationIssue
	ValidateCommandCatalogJSON(json []byte) []protocol.ValidationIssue
}

type EntityFunctions struct {
	pgStore        store.EntityStore
	log            *logging.Logger
	protoValidator ProtocolValidator
	publisher      Publisher
}

func NewEntityFunctions(pgStore store.EntityStore, log *logging.Logger, protoValidator ProtocolValidator, publishers ...Publisher) EntityFunctions {
	if protoValidator == nil {
		protoValidator = noopProtocolValidator{}
	}
	return EntityFunctions{pgStore: pgStore, log: log, protoValidator: protoValidator, publisher: publisherOrNop(publishers)}
}

func (f EntityFunctions) CreateEntity(ctx context.Context, entity *model.Entity) error {
	if err := validateEntityModel(entity); err != nil {
		return err
	}
	now := time.Now().UTC()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	if entity.UpdatedAt.IsZero() {
		entity.UpdatedAt = now
	}
	if entity.JSON == nil {
		entity.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateEntity(entity); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	f.log.InfoContext(ctx, "entity", "creating entity", logging.String("entity_id", entity.EntityID), logging.String("entity_type", string(entity.Type)))
	if err := f.pgStore.CreateEntity(ctx, entity); err != nil {
		return err
	}
	publishEntity(ctx, f.publisher, "created", entity)
	return nil
}

func (f EntityFunctions) GetEntity(ctx context.Context, entityID string) (*model.Entity, error) {
	if entityID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	return f.pgStore.GetEntity(ctx, entityID)
}

func (f EntityFunctions) ListEntities(ctx context.Context, params store.EntityListParams) (store.EntityListResult, error) {
	return f.pgStore.ListEntities(ctx, params)
}

func (f EntityFunctions) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	if err := validateEntityModel(entity); err != nil {
		return err
	}
	if entity.JSON == nil {
		entity.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateEntity(entity); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	entity.UpdatedAt = time.Now().UTC()
	f.log.InfoContext(ctx, "entity", "updating entity", logging.String("entity_id", entity.EntityID), logging.String("entity_type", string(entity.Type)))
	if err := f.pgStore.UpdateEntity(ctx, entity); err != nil {
		return err
	}
	publishEntity(ctx, f.publisher, "updated", entity)
	return nil
}

func (f EntityFunctions) DeleteEntity(ctx context.Context, entityID string) error {
	if entityID == "" {
		return model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	f.log.InfoContext(ctx, "entity", "deleting entity", logging.String("entity_id", entityID))
	entity, err := f.pgStore.GetEntity(ctx, entityID)
	if err != nil {
		return err
	}
	if err := f.pgStore.DeleteEntity(ctx, entityID); err != nil {
		return err
	}
	publishEntity(ctx, f.publisher, "deleted", entity)
	return nil
}

func (f EntityFunctions) UpsertEntity(ctx context.Context, entity *model.Entity) error {
	if err := validateEntityModel(entity); err != nil {
		return err
	}
	if entity.JSON == nil {
		entity.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateEntity(entity); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	now := time.Now().UTC()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	entity.UpdatedAt = now
	f.log.InfoContext(ctx, "entity", "upserting entity", logging.String("entity_id", entity.EntityID), logging.String("entity_type", string(entity.Type)))
	if err := f.pgStore.UpsertEntity(ctx, entity); err != nil {
		return err
	}
	publishEntity(ctx, f.publisher, "updated", entity)
	return nil
}
