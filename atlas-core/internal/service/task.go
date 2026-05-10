package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/protocolvalidation"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

const commandCatalogObjectID = "command_catalog"

type TaskFunctions struct {
	taskStore   ports.TaskStore
	entityStore ports.EntityStore
	objectStore ports.ObjectStore
	idemStore   ports.IdempotencyStore
	log         *logging.Logger
	validator   protocolvalidation.JSONValidator
}

func (f TaskFunctions) protocolValidator() protocolvalidation.JSONValidator {
	if f.validator != nil {
		return f.validator
	}
	return protocolvalidation.NewRunner()
}

func (f TaskFunctions) createTaskInner(ctx context.Context, task *model.Task) error {
	f.log.InfoContext(ctx, "task", "creating task", logging.String("task_id", task.TaskID), logging.String("command_catalog_object_id", task.CommandCatalogObjectID))
	return f.taskStore.CreateTask(ctx, task)
}

func (f TaskFunctions) ensureTaskCreated(ctx context.Context, task *model.Task) error {
	if err := f.createTaskInner(ctx, task); err != nil {
		if !errors.Is(err, model.ErrConflict) {
			return err
		}
		if _, getErr := f.taskStore.GetTask(ctx, task.TaskID); getErr != nil {
			return getErr
		}
	}
	return nil
}

func (f TaskFunctions) CreateTask(ctx context.Context, task *model.Task, opts ...IdempotencyOption) error {
	if err := validateTaskModel(task); err != nil {
		return err
	}
	if err := f.protocolValidator().NormalizeTask(task, protocolvalidation.OperationCreate); err != nil {
		return err
	}
	if err := f.validateTaskSemantics(ctx, task, protocolvalidation.OperationCreate); err != nil {
		return err
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}
	idem := resolveIdempotency(opts)
	if idem.key != "" {
		record, claimed, err := f.idemStore.TryBegin(ctx, "task_create", idem.key, task.TaskID)
		if err != nil {
			return err
		}
		if !claimed {
			if record.ResourceID != task.TaskID {
				return model.NewFieldError("CONFLICT",
					fmt.Sprintf("idempotency key %q already used for task %q", idem.key, record.ResourceID),
					"idempotency_key")
			}
			if record.Status == ports.IdempotencyStatusCompleted {
				f.log.InfoContext(ctx, "task", "idempotent create replay",
					logging.String("task_id", task.TaskID),
					logging.String("idempotency_key", idem.key),
				)
				return nil
			}
		}
		createFn := f.ensureTaskCreated
		if claimed {
			createFn = f.createTaskInner
		}
		if err := createFn(ctx, task); err != nil {
			if claimed {
				if markErr := f.idemStore.MarkFailed(ctx, "task_create", idem.key); markErr != nil {
					return errors.Join(err, markErr)
				}
			}
			return err
		}
		return f.idemStore.MarkCompleted(ctx, "task_create", idem.key)
	}

	return f.createTaskInner(ctx, task)
}

func (f TaskFunctions) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	if taskID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	return f.taskStore.GetTask(ctx, taskID)
}

func (f TaskFunctions) ListTasks(ctx context.Context, filters ...ports.TaskFilter) ([]model.Task, error) {
	return f.taskStore.ListTasks(ctx, filters...)
}

func (f TaskFunctions) UpdateTask(ctx context.Context, task *model.Task) error {
	if err := validateTaskModel(task); err != nil {
		return err
	}
	if err := f.protocolValidator().NormalizeTask(task, protocolvalidation.OperationUpdate); err != nil {
		return err
	}
	if err := f.validateTaskSemantics(ctx, task, protocolvalidation.OperationUpdate); err != nil {
		return err
	}
	task.UpdatedAt = time.Now().UTC()
	f.log.InfoContext(ctx, "task", "updating task", logging.String("task_id", task.TaskID), logging.String("command_catalog_object_id", task.CommandCatalogObjectID))
	return f.taskStore.UpdateTask(ctx, task)
}

func (f TaskFunctions) DeleteTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	f.log.InfoContext(ctx, "task", "deleting task", logging.String("task_id", taskID))
	return f.taskStore.DeleteTask(ctx, taskID)
}

func (f TaskFunctions) UpsertTask(ctx context.Context, task *model.Task) error {
	if err := validateTaskModel(task); err != nil {
		return err
	}
	if err := f.protocolValidator().NormalizeTask(task, protocolvalidation.OperationUpsert); err != nil {
		return err
	}
	if err := f.validateTaskSemantics(ctx, task, protocolvalidation.OperationUpsert); err != nil {
		return err
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	f.log.InfoContext(ctx, "task", "upserting task", logging.String("task_id", task.TaskID), logging.String("command_catalog_object_id", task.CommandCatalogObjectID))
	return f.taskStore.UpsertTask(ctx, task)
}

func (f TaskFunctions) validateTaskSemantics(ctx context.Context, task *model.Task, op protocolvalidation.Operation) error {
	commandType, parameters, err := taskCommandPayload(task.JSON)
	if err != nil {
		return err
	}
	if err := f.validateTaskAsset(ctx, task.AssetID, commandType); err != nil {
		return err
	}
	catalog, _, err := f.validateCommandCatalogObject(ctx, task, op)
	if err != nil {
		return err
	}
	if catalog == nil {
		return nil
	}
	return validateTaskParametersAgainstCatalog(catalog, commandType, parameters)
}

func (f TaskFunctions) validateTaskAsset(ctx context.Context, assetID, commandType string) error {
	entity, err := f.entityStore.GetEntity(ctx, assetID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return model.NewFieldError("INVALID_INPUT", "asset_id must reference an existing asset", "asset_id")
		}
		return err
	}
	if entity.Type != model.EntityTypeAsset {
		return model.NewFieldError("INVALID_INPUT", "asset_id must reference an asset", "asset_id")
	}
	var payload struct {
		Components struct {
			SupportedCommands struct {
				Commands []string `json:"commands"`
			} `json:"supported_commands"`
		} `json:"components"`
	}
	if err := json.Unmarshal(entity.JSON, &payload); err != nil {
		return model.NewFieldError("INVALID_INPUT", "asset_id must reference an asset with valid supported_commands", "asset_id")
	}
	for _, supported := range payload.Components.SupportedCommands.Commands {
		if supported == commandType {
			return nil
		}
	}
	return model.NewFieldError("INVALID_INPUT", "asset_id does not support the requested command", "asset_id")
}

func (f TaskFunctions) validateCommandCatalogObject(ctx context.Context, task *model.Task, op protocolvalidation.Operation) (*model.Object, bool, error) {
	objectID := task.CommandCatalogObjectID
	obj, err := f.objectStore.GetObject(ctx, objectID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			// Catalog is missing entirely — allow if this is a no-op on an
			// existing task that already held the same reference.
			allowed, allowErr := f.allowLegacyOrMissingCatalogReference(ctx, task, op, "")
			if allowErr != nil {
				return nil, false, allowErr
			}
			if allowed {
				return nil, true, nil
			}
			return nil, false, model.NewFieldError("INVALID_INPUT", "command_catalog_object_id must reference an existing command_catalog document object", "command_catalog_object_id")
		}
		return nil, false, err
	}
	if obj.ObjectID != commandCatalogObjectID || obj.Type != model.ObjectTypeDocument {
		// Object exists but isn't the canonical command_catalog document — allow
		// only if it's a legacy "command_catalog" typed object and the write is a
		// no-op on an existing task.
		allowed, getErr := f.allowLegacyOrMissingCatalogReference(ctx, task, op, obj.Type)
		if getErr != nil {
			return nil, false, getErr
		}
		if allowed {
			return nil, true, nil
		}
		return nil, false, model.NewFieldError("INVALID_INPUT", "command_catalog_object_id must reference the command_catalog document object", "command_catalog_object_id")
	}
	return obj, false, nil
}

// allowLegacyOrMissingCatalogReference determines whether an update/upsert task
// can tolerate a missing or legacy command catalog reference.
//
//   - objectType == "": the catalog object was not found (ErrNotFound).
//   - objectType != "": the object was found but has a non-standard ID or type;
//     tolerance is only considered when the found type is "command_catalog" (legacy).
//
// In either case tolerance is only granted for no-op writes where the existing
// task already held the same catalog reference and the command type and parameters
// are unchanged.
func (f TaskFunctions) allowLegacyOrMissingCatalogReference(ctx context.Context, task *model.Task, op protocolvalidation.Operation, objectType model.ObjectType) (bool, error) {
	if op == protocolvalidation.OperationCreate {
		return false, nil
	}
	// Legacy path: object exists but isn't the canonical document — only allow
	// tolerance if the found type is "command_catalog".
	if objectType != "" && string(objectType) != "command_catalog" {
		return false, nil
	}
	existingTask, err := f.taskStore.GetTask(ctx, task.TaskID)
	if err != nil {
		// New tasks should fall back to the standard validation error instead of
		// silently accepting a missing or legacy command catalog reference.
		if errors.Is(err, model.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if existingTask.CommandCatalogObjectID != task.CommandCatalogObjectID {
		return false, nil
	}
	// Tolerance only applies to no-op writes against an already-pinned catalog
	// row; any change to command type or parameters must be re-validated against
	// a current catalog and is rejected here.
	existingCommandType, existingParameters, err := taskCommandPayload(existingTask.JSON)
	if err != nil {
		return false, nil
	}
	commandType, parameters, err := taskCommandPayload(task.JSON)
	if err != nil {
		return false, err
	}
	return existingCommandType == commandType && reflect.DeepEqual(existingParameters, parameters), nil
}

func taskCommandPayload(data []byte) (string, map[string]any, error) {
	var payload struct {
		Components struct {
			Command struct {
				Type string `json:"type"`
			} `json:"command"`
			Parameters map[string]any `json:"parameters"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", nil, model.NewFieldError("INVALID_INPUT", "json must be a valid task object", "json")
	}
	if payload.Components.Parameters == nil {
		payload.Components.Parameters = map[string]any{}
	}
	return payload.Components.Command.Type, payload.Components.Parameters, nil
}

func validateTaskParametersAgainstCatalog(catalog *model.Object, commandType string, parameters map[string]any) error {
	var payload map[string]any
	if err := json.Unmarshal(catalog.JSON, &payload); err != nil {
		return model.NewFieldError("INVALID_INPUT", "command_catalog_object_id must reference a valid command catalog payload", "command_catalog_object_id")
	}
	commands, ok := payload["commands"].(map[string]any)
	if !ok {
		return model.NewFieldError("INVALID_INPUT", "command_catalog_object_id must reference a command catalog with a commands object", "command_catalog_object_id")
	}
	commandValue, ok := commands[commandType]
	if !ok {
		return model.NewFieldError("INVALID_INPUT", "command type must exist in the command catalog", "json.components.command.type")
	}
	command, ok := commandValue.(map[string]any)
	if !ok {
		return model.NewFieldError("INVALID_INPUT", "command catalog entry must be an object", "command_catalog_object_id")
	}
	parametersSchema, ok := command["parameters_schema"]
	if !ok || parametersSchema == nil {
		return model.NewFieldError("INVALID_INPUT", "command catalog entry must include parameters_schema", "command_catalog_object_id")
	}
	parametersSchemaObject, ok := parametersSchema.(map[string]any)
	if !ok {
		return model.NewFieldError("INVALID_INPUT", "command catalog entry parameters_schema must be an object", "command_catalog_object_id")
	}
	if err := protocolvalidation.ValidateCommandSchema(parametersSchemaObject, parameters); err != nil {
		return err
	}
	return nil
}
