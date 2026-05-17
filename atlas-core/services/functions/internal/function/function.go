package function

import (
	"atlas.local/protocol"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
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

func (f EntityFunctions) ListEntities(ctx context.Context, filters ...store.EntityFilter) ([]model.Entity, error) {
	return f.pgStore.ListEntities(ctx, filters...)
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

type ObjectFunctions struct {
	idemStore      store.IdempotencyStore
	log            *logging.Logger
	protoValidator ProtocolValidator
	gateway        ObjectGateway
	publisher      Publisher
}

var errDecodeObjectManifest = errors.New("decode object manifest")

var objectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// IdempotencyOption attaches an idempotency key to a mutating function call.
// When provided, the function tries to claim the key before performing the
// operation. A repeated call with the same key against the same resource
// returns nil (the original effect). A call with the same key against a
// different resource returns model.ErrConflict.
type IdempotencyOption func(*idempotencyOptions)

type idempotencyOptions struct {
	key string
}

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

// WithIdempotencyKey returns an IdempotencyOption that scopes a mutation to
// the given client-supplied key. Empty keys disable the check.
func WithIdempotencyKey(key string) IdempotencyOption {
	return func(o *idempotencyOptions) { o.key = key }
}

func resolveIdempotency(opts []IdempotencyOption) idempotencyOptions {
	var o idempotencyOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

func failClaimedIdempotency(ctx context.Context, idemStore store.IdempotencyStore, claimed bool, scope, key string, err error) error {
	if claimed {
		if markErr := idemStore.MarkFailed(ctx, scope, key); markErr != nil {
			return errors.Join(err, markErr)
		}
	}
	return err
}

func NewObjectFunctions(gateway ObjectGateway, idemStore store.IdempotencyStore, log *logging.Logger, protoValidator ProtocolValidator, publishers ...Publisher) ObjectFunctions {
	return ObjectFunctions{
		idemStore:      idemStore,
		log:            log,
		protoValidator: protoValidator,
		gateway:        gateway,
		publisher:      publisherOrNop(publishers),
	}
}

func (f ObjectFunctions) CreateObject(ctx context.Context, obj *model.Object, opts ...IdempotencyOption) error {
	if err := validateObjectModel(obj); err != nil {
		return err
	}
	now := time.Now().UTC()
	if obj.CreatedAt.IsZero() {
		obj.CreatedAt = now
	}
	if obj.UpdatedAt.IsZero() {
		obj.UpdatedAt = now
	}
	if obj.JSON == nil {
		obj.JSON = []byte("{}")
	}

	idem := resolveIdempotency(opts)
	if idem.key != "" {
		record, claimed, err := f.idemStore.TryBegin(ctx, "object_create", idem.key, obj.ObjectID)
		if err != nil {
			return err
		}
		if !claimed {
			if record.ResourceID != obj.ObjectID {
				return model.NewFieldError("CONFLICT",
					fmt.Sprintf("idempotency key %q already used for object %q", idem.key, record.ResourceID),
					"idempotency_key")
			}
			if record.Status == store.IdempotencyStatusCompleted {
				f.log.InfoContext(ctx, "object", "idempotent create replay",
					logging.String("object_id", obj.ObjectID),
					logging.String("idempotency_key", idem.key),
				)
				return nil
			}
		}
		if issues := f.protoValidator.ValidateObject(obj); len(issues) > 0 {
			return failClaimedIdempotency(ctx, f.idemStore, claimed, "object_create", idem.key, protocolvalidation.NewValidationError(issues))
		}
		createFn := f.gateway.EnsureObjectCreated
		if claimed {
			createFn = f.gateway.CreateObject
		}
		if err := createFn(ctx, obj); err != nil {
			return failClaimedIdempotency(ctx, f.idemStore, claimed, "object_create", idem.key, err)
		}
		if err := f.idemStore.MarkCompleted(ctx, "object_create", idem.key); err != nil {
			return err
		}
		publishObject(ctx, f.publisher, "created", obj)
		return nil
	}

	if issues := f.protoValidator.ValidateObject(obj); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	if err := f.createObjectInner(ctx, obj); err != nil {
		return err
	}
	publishObject(ctx, f.publisher, "created", obj)
	return nil
}

func (f ObjectFunctions) createObjectInner(ctx context.Context, obj *model.Object) error {
	f.log.InfoContext(ctx, "object", "creating object", logging.String("object_id", obj.ObjectID), logging.String("object_type", string(obj.Type)))
	return f.gateway.CreateObject(ctx, obj)
}

func (f ObjectFunctions) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	if objectID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	return f.gateway.GetObject(ctx, objectID)
}

func (f ObjectFunctions) ListObjects(ctx context.Context, filters ...store.ObjectFilter) ([]model.Object, error) {
	return f.gateway.ListObjects(ctx, filters...)
}

func (f ObjectFunctions) UpdateObject(ctx context.Context, obj *model.Object) error {
	if err := validateObjectModel(obj); err != nil {
		return err
	}
	if obj.JSON == nil {
		obj.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateObject(obj); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	obj.UpdatedAt = time.Now().UTC()
	f.log.InfoContext(ctx, "object", "updating object", logging.String("object_id", obj.ObjectID), logging.String("object_type", string(obj.Type)))
	if err := f.gateway.UpdateObject(ctx, obj); err != nil {
		return err
	}
	publishObject(ctx, f.publisher, "updated", obj)
	return nil
}

func (f ObjectFunctions) DeleteObject(ctx context.Context, objectID string) error {
	if objectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	obj, err := f.gateway.GetObject(ctx, objectID)
	if err != nil {
		return err
	}
	f.log.InfoContext(ctx, "object", "deleting object", logging.String("object_id", objectID), logging.String("object_type", string(obj.Type)))
	if err := f.gateway.DeleteObject(ctx, objectID); err != nil {
		return err
	}
	publishObject(ctx, f.publisher, "deleted", obj)
	return nil
}

func (f ObjectFunctions) UpsertObject(ctx context.Context, obj *model.Object) error {
	if err := validateObjectModel(obj); err != nil {
		return err
	}
	now := time.Now().UTC()
	if obj.CreatedAt.IsZero() {
		obj.CreatedAt = now
	}
	obj.UpdatedAt = now
	if obj.JSON == nil {
		obj.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateObject(obj); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	f.log.InfoContext(ctx, "object", "upserting object", logging.String("object_id", obj.ObjectID), logging.String("object_type", string(obj.Type)))
	if err := f.gateway.UpsertObject(ctx, obj); err != nil {
		return err
	}
	publishObject(ctx, f.publisher, "updated", obj)
	return nil
}

func (f ObjectFunctions) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	if objectID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	return f.gateway.GetObjectManifest(ctx, objectID)
}

func (f ObjectFunctions) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest) error {
	if objectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	if manifest == nil {
		return model.NewFieldError("INVALID_INPUT", "manifest is required", "manifest")
	}
	if _, err := f.gateway.GetObject(ctx, objectID); err != nil {
		return err
	}
	manifest = model.NormalizeManifest(manifest)
	if err := f.gateway.UpdateObjectManifest(ctx, objectID, manifest); err != nil {
		return err
	}
	f.log.InfoContext(ctx, "object", "updated object manifest", logging.String("object_id", objectID), logging.String("manifest_version", manifest.Version))
	f.publishObjectMutation(ctx, "updated", objectID)
	return nil
}

func (f ObjectFunctions) WriteFile(ctx context.Context, objectID, filename string, data []byte) (ManifestResult, error) {
	f.log.InfoContext(ctx, "object", "writing object file", logging.String("object_id", objectID), logging.String("filename", filename), logging.Any("size", len(data)))
	result, err := f.gateway.WriteFile(ctx, objectID, filename, data)
	if err != nil {
		return ManifestResult{}, err
	}
	f.publishObjectMutation(ctx, "updated", objectID)
	return result, nil
}

func (f ObjectFunctions) AppendFile(ctx context.Context, objectID, filename string, data []byte) (ManifestResult, error) {
	f.log.InfoContext(ctx, "object", "appending object file", logging.String("object_id", objectID), logging.String("filename", filename), logging.Any("size", len(data)))
	result, err := f.gateway.AppendFile(ctx, objectID, filename, data)
	if err != nil {
		return ManifestResult{}, err
	}
	f.publishObjectMutation(ctx, "updated", objectID)
	return result, nil
}

func (f ObjectFunctions) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	return f.gateway.ReadFile(ctx, objectID, filename)
}

func (f ObjectFunctions) DeleteFile(ctx context.Context, objectID, filename string) (ManifestResult, error) {
	f.log.InfoContext(ctx, "object", "deleting object file", logging.String("object_id", objectID), logging.String("filename", filename))
	result, err := f.gateway.DeleteFile(ctx, objectID, filename)
	if err != nil {
		return ManifestResult{}, err
	}
	f.publishObjectMutation(ctx, "updated", objectID)
	return result, nil
}

func (f ObjectFunctions) ListFiles(ctx context.Context, objectID string) ([]string, error) {
	if err := validateObjectID(objectID); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	return f.gateway.ListFiles(ctx, objectID)
}

func (f ObjectFunctions) Reconcile(ctx context.Context) error {
	f.log.InfoContext(ctx, "object_reconcile", "starting object reconciliation")
	err := f.gateway.Reconcile(ctx)
	if err == nil {
		f.log.InfoContext(ctx, "object_reconcile", "finished object reconciliation")
	}
	return err
}

func (f ObjectFunctions) publishObjectMutation(ctx context.Context, operation, objectID string) {
	object, err := f.gateway.GetObject(ctx, objectID)
	if err != nil {
		f.log.WarnContext(ctx, "object", "publishing object mutation without snapshot", logging.String("object_id", objectID), logging.ErrorField(err))
		publishObjectID(ctx, f.publisher, operation, objectID)
		return
	}
	publishObject(ctx, f.publisher, operation, object)
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
				return model.NewFieldError("CONFLICT",
					fmt.Sprintf("idempotency key %q already used for task %q", idem.key, record.ResourceID),
					"idempotency_key")
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

func (f TaskFunctions) ListTasks(ctx context.Context, filters ...store.TaskFilter) ([]model.Task, error) {
	return f.taskStore.ListTasks(ctx, filters...)
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
		return model.NewFieldError("INTERNAL", "task JSON is corrupt", "json")
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

func (f ObservationFunctions) ListObservations(ctx context.Context, filters ...store.ObservationFilter) ([]model.Observation, error) {
	return f.pgStore.ListObservations(ctx, filters...)
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

func requireModel[T any](value *T, field string) error {
	if value == nil {
		return model.NewFieldError("INVALID_INPUT", field+" is required", field)
	}
	return nil
}

func validateEntityModel(entity *model.Entity) error {
	if err := requireModel(entity, "entity"); err != nil {
		return err
	}
	if entity.EntityID == "" {
		return model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	if len(entity.EntityID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "entity_id must be 1-50 characters", "entity_id")
	}
	if entity.Type != model.EntityTypeAsset && entity.Type != model.EntityTypeTrack && entity.Type != model.EntityTypeGeofeature {
		return model.NewFieldError("INVALID_INPUT", "type must be asset, track, or geofeature", "type")
	}
	return nil
}

func validateObjectModel(obj *model.Object) error {
	if err := requireModel(obj, "object"); err != nil {
		return err
	}
	if obj.ObjectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	if len(obj.ObjectID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "object_id must be 1-50 characters", "object_id")
	}
	if obj.Type == "" {
		return model.NewFieldError("INVALID_INPUT", "type is required", "type")
	}
	if !isKnownObjectType(obj.Type) {
		return model.NewFieldError("INVALID_INPUT", "type must be command_catalog, log, or photo", "type")
	}
	if obj.OwnerType != model.OwnerTypeEntity && obj.OwnerType != model.OwnerTypeObservation && obj.OwnerType != model.OwnerTypeTask && obj.OwnerType != model.OwnerTypeSystem {
		return model.NewFieldError("INVALID_INPUT", "owner_type must be entity, observation, task, or system", "owner_type")
	}
	if obj.OwnerID == "" {
		return model.NewFieldError("INVALID_INPUT", "owner_id is required", "owner_id")
	}
	if err := validateObjectID(obj.ObjectID); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	return nil
}

func validateTaskModel(task *model.Task) error {
	if err := requireModel(task, "task"); err != nil {
		return err
	}
	if task.TaskID == "" {
		return model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	if len(task.TaskID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "task_id must be 1-50 characters", "task_id")
	}
	if task.Status != model.TaskStatusPending && task.Status != model.TaskStatusAcknowledged && task.Status != model.TaskStatusCompleted && task.Status != model.TaskStatusFailed {
		return model.NewFieldError("INVALID_INPUT", "status must be pending, acknowledged, completed, or failed", "status")
	}
	if task.AssetID == "" {
		return model.NewFieldError("INVALID_INPUT", "asset_id is required", "asset_id")
	}
	if task.CommandCatalogObjectID == "" {
		return model.NewFieldError("INVALID_INPUT", "command_catalog_object_id is required", "command_catalog_object_id")
	}
	return nil
}

func validateObservationModel(obs *model.Observation) error {
	if err := requireModel(obs, "observation"); err != nil {
		return err
	}
	if obs.ObservationID == "" {
		return model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	if len(obs.ObservationID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "observation_id must be 1-50 characters", "observation_id")
	}
	if obs.SourceAssetID == "" {
		return model.NewFieldError("INVALID_INPUT", "source_asset_id is required", "source_asset_id")
	}
	return nil
}

func isKnownObjectType(objectType model.ObjectType) bool {
	for _, known := range model.KnownObjectTypes() {
		if known == objectType {
			return true
		}
	}
	return false
}

func validateObjectID(objectID string) error {
	if objectID == "" {
		return fmt.Errorf("object_id is required")
	}
	if objectID == "." || objectID == ".." {
		return fmt.Errorf("invalid path: object_id must not be '.' or '..'")
	}
	if objectID == "manifest.json" {
		return fmt.Errorf("invalid path: object_id is reserved")
	}
	if filepath.IsAbs(objectID) || strings.ContainsAny(objectID, `/\\`) {
		return fmt.Errorf("invalid path: object_id contains path separators")
	}
	if !objectIDPattern.MatchString(objectID) {
		return fmt.Errorf("invalid path: object_id must use only letters, numbers, '_' or '-'")
	}
	return nil
}
