package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type taskRuntimeJSON struct {
	Components taskRuntimeComponents `json:"components"`
}

type taskRuntimeComponents struct {
	Command    taskRuntimeCommand `json:"command"`
	Parameters map[string]any     `json:"parameters"`
}

type taskRuntimeCommand struct {
	Type string `json:"type"`
}
type TaskFunctions struct {
	taskStore      store.TaskStore
	objectStore    store.ObjectStore
	entityStore    store.EntityStore
	idemStore      store.IdempotencyStore
	log            *logging.Logger
	protoValidator ProtocolValidator
	publisher      Publisher
}

func NewTaskFunctions(taskStore store.TaskStore, objectStore store.ObjectStore, entityStore store.EntityStore, idemStore store.IdempotencyStore, log *logging.Logger, protoValidator ProtocolValidator, publishers ...Publisher) TaskFunctions {
	return TaskFunctions{taskStore: taskStore, objectStore: objectStore, entityStore: entityStore, idemStore: idemStore, log: log, protoValidator: protoValidator, publisher: publisherOrNop(publishers)}
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
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}
	if task.JSON == nil {
		task.JSON = []byte("{}")
	}

	idem := resolveIdempotency(opts)
	if idem.key != "" {
		record, claimed, err := f.idemStore.TryBegin(ctx, "task_create", idem.key, task.TaskID)
		if err != nil {
			return err
		}
		if !claimed {
			if record.ResourceID != task.TaskID {
				return model.NewIdempotencyKeyConflictError(idem.key, record.ResourceID)
			}
			if record.Status == store.IdempotencyStatusCompleted {
				f.log.InfoContext(ctx, "task", "idempotent create replay",
					logging.String("task_id", task.TaskID),
					logging.String("idempotency_key", idem.key),
				)
				return nil
			}
		}
		if err := f.validateTaskRuntime(ctx, task); err != nil {
			return failClaimedIdempotency(ctx, f.idemStore, claimed, "task_create", idem.key, err)
		}
		createFn := f.ensureTaskCreated
		if claimed {
			createFn = f.createTaskInner
		}
		if err := createFn(ctx, task); err != nil {
			return failClaimedIdempotency(ctx, f.idemStore, claimed, "task_create", idem.key, err)
		}
		if err := f.idemStore.MarkCompleted(ctx, "task_create", idem.key); err != nil {
			return err
		}
		publishTask(ctx, f.publisher, "created", task)
		return nil
	}

	if err := f.validateTaskRuntime(ctx, task); err != nil {
		return err
	}
	if err := f.createTaskInner(ctx, task); err != nil {
		return err
	}
	publishTask(ctx, f.publisher, "created", task)
	return nil
}

func (f TaskFunctions) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	if taskID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	return f.taskStore.GetTask(ctx, taskID)
}

func (f TaskFunctions) ListTasks(ctx context.Context, params store.TaskListParams) (store.TaskListResult, error) {
	return f.taskStore.ListTasks(ctx, params)
}

func (f TaskFunctions) UpdateTask(ctx context.Context, task *model.Task) error {
	if err := validateTaskModel(task); err != nil {
		return err
	}
	if task.JSON == nil {
		task.JSON = []byte("{}")
	}
	if err := f.validateTaskRuntime(ctx, task); err != nil {
		return err
	}
	task.UpdatedAt = time.Now().UTC()
	f.log.InfoContext(ctx, "task", "updating task", logging.String("task_id", task.TaskID), logging.String("command_catalog_object_id", task.CommandCatalogObjectID))
	if err := f.taskStore.UpdateTask(ctx, task); err != nil {
		return err
	}
	publishTask(ctx, f.publisher, "updated", task)
	return nil
}

func (f TaskFunctions) DeleteTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	f.log.InfoContext(ctx, "task", "deleting task", logging.String("task_id", taskID))
	task, err := f.taskStore.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if err := f.taskStore.DeleteTask(ctx, taskID); err != nil {
		return err
	}
	publishTask(ctx, f.publisher, "deleted", task)
	return nil
}

func (f TaskFunctions) UpsertTask(ctx context.Context, task *model.Task) error {
	if err := validateTaskModel(task); err != nil {
		return err
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	if task.JSON == nil {
		task.JSON = []byte("{}")
	}
	if err := f.validateTaskRuntime(ctx, task); err != nil {
		return err
	}
	f.log.InfoContext(ctx, "task", "upserting task", logging.String("task_id", task.TaskID), logging.String("command_catalog_object_id", task.CommandCatalogObjectID))
	if err := f.taskStore.UpsertTask(ctx, task); err != nil {
		return err
	}
	publishTask(ctx, f.publisher, "updated", task)
	return nil
}

func (f TaskFunctions) validateTaskRuntime(ctx context.Context, task *model.Task) error {
	if issues := f.protoValidator.ValidateTask(task); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}

	asset, err := f.entityStore.GetEntity(ctx, task.AssetID)
	if err != nil {
		return err
	}
	if asset.Type != model.EntityTypeAsset {
		return model.NewFieldError("INVALID_INPUT", "asset_id must reference an asset entity", "asset_id")
	}

	catalogObj, err := f.objectStore.GetObject(ctx, task.CommandCatalogObjectID)
	if err != nil {
		return err
	}
	if catalogObj.Type != model.ObjectTypeCommandCatalog {
		return model.NewFieldError("INVALID_INPUT", "command_catalog_object_id must reference a command_catalog object", "command_catalog_object_id")
	}

	if issues := f.protoValidator.ValidateCommandCatalogJSON(catalogObj.JSON); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}

	var taskJSON taskRuntimeJSON
	if err := json.Unmarshal(task.JSON, &taskJSON); err != nil {
		return model.NewFieldError("INVALID_INPUT", "task JSON is corrupt", "json")
	}
	commandType := taskJSON.Components.Command.Type

	var catalogJSON map[string]any
	if err := json.Unmarshal(catalogObj.JSON, &catalogJSON); err != nil {
		return model.NewFieldError("INTERNAL", "command catalog JSON is corrupt", "command_catalog_object_id")
	}
	commands, _ := catalogJSON["commands"].([]any)
	var catalogCmd map[string]any
	for _, c := range commands {
		if cmd, ok := c.(map[string]any); ok {
			if id, _ := cmd["id"].(string); id == commandType {
				catalogCmd = cmd
				break
			}
		}
	}
	if catalogCmd == nil {
		return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("command %q not found in catalog", commandType), "json.components.command.type")
	}

	taskParams := taskJSON.Components.Parameters
	schema, _ := catalogCmd["parameters_schema"].(map[string]any)
	if err := validateTaskParamsAgainstSchema(taskParams, schema); err != nil {
		return err
	}

	var assetJSON map[string]any
	if err := json.Unmarshal(asset.JSON, &assetJSON); err != nil {
		return model.NewFieldError("INTERNAL", "target asset JSON is corrupt", "asset_id")
	}
	assetComponents, _ := assetJSON["components"].(map[string]any)
	supportedCmds, _ := assetComponents["supported_commands"].(map[string]any)
	if supportedCmds == nil {
		return model.NewFieldError("INVALID_INPUT", "target asset does not declare supported_commands", "asset_id")
	}
	cmdList, _ := supportedCmds["commands"].([]any)
	supported := false
	for _, c := range cmdList {
		if s, ok := c.(string); ok && s == commandType {
			supported = true
			break
		}
	}
	if !supported {
		return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("target asset does not support command %q", commandType), "json.components.command.type")
	}

	return nil
}

func validateTaskParamsAgainstSchema(params map[string]any, schema map[string]any) error {
	for paramName := range params {
		if schema == nil {
			return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("parameter %q is not defined in command catalog", paramName), "json.components.parameters."+paramName)
		}
		if _, ok := schema[paramName]; !ok {
			return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("parameter %q is not defined in command catalog", paramName), "json.components.parameters."+paramName)
		}
	}
	for paramName, schemaVal := range schema {
		paramDef, ok := schemaVal.(map[string]any)
		if !ok {
			continue
		}
		required, _ := paramDef["required"].(bool)
		paramType, _ := paramDef["type"].(string)
		paramValue, exists := params[paramName]

		if !exists {
			if required {
				return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("required parameter %q is missing", paramName), "json.components.parameters."+paramName)
			}
			continue
		}
		switch paramType {
		case "string":
			if _, ok := paramValue.(string); !ok {
				return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("parameter %q must be a string", paramName), "json.components.parameters."+paramName)
			}
		case "number":
			if _, ok := paramValue.(float64); !ok {
				return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("parameter %q must be a number", paramName), "json.components.parameters."+paramName)
			}
		case "boolean":
			if _, ok := paramValue.(bool); !ok {
				return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("parameter %q must be a boolean", paramName), "json.components.parameters."+paramName)
			}
		case "object":
			if _, ok := paramValue.(map[string]any); !ok {
				return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("parameter %q must be an object", paramName), "json.components.parameters."+paramName)
			}
		case "array":
			if _, ok := paramValue.([]any); !ok {
				return model.NewFieldError("INVALID_INPUT", fmt.Sprintf("parameter %q must be an array", paramName), "json.components.parameters."+paramName)
			}
		}
	}
	return nil
}
