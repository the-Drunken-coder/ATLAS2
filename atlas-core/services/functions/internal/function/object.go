package function

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/functions/internal/gateway"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type ObjectFunctions struct {
	idemStore      store.IdempotencyStore
	log            *logging.Logger
	protoValidator ProtocolValidator
	gateway        gateway.ObjectGateway
	publisher      Publisher
}

var errDecodeObjectManifest = errors.New("decode object manifest")

func NewObjectFunctions(gw gateway.ObjectGateway, idemStore store.IdempotencyStore, log *logging.Logger, protoValidator ProtocolValidator, publishers ...Publisher) ObjectFunctions {
	return ObjectFunctions{
		idemStore:      idemStore,
		log:            log,
		protoValidator: protoValidator,
		gateway:        gw,
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

func (f ObjectFunctions) ListObjects(ctx context.Context, params store.ObjectListParams) (store.ObjectListResult, error) {
	return f.gateway.ListObjects(ctx, params)
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

func (f ObjectFunctions) WriteFile(ctx context.Context, objectID, filename string, data []byte) (gateway.ManifestResult, error) {
	if err := validateObjectID(objectID); err != nil {
		return gateway.ManifestResult{}, model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	f.log.InfoContext(ctx, "object", "writing object file", logging.String("object_id", objectID), logging.String("filename", filename), logging.Any("size", len(data)))
	result, err := f.gateway.WriteFile(ctx, objectID, filename, data)
	if err != nil {
		return gateway.ManifestResult{}, err
	}
	f.publishObjectMutation(ctx, "updated", objectID)
	return result, nil
}

func (f ObjectFunctions) AppendFile(ctx context.Context, objectID, filename string, data []byte) (gateway.ManifestResult, error) {
	if err := validateObjectID(objectID); err != nil {
		return gateway.ManifestResult{}, model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	f.log.InfoContext(ctx, "object", "appending object file", logging.String("object_id", objectID), logging.String("filename", filename), logging.Any("size", len(data)))
	result, err := f.gateway.AppendFile(ctx, objectID, filename, data)
	if err != nil {
		return gateway.ManifestResult{}, err
	}
	f.publishObjectMutation(ctx, "updated", objectID)
	return result, nil
}

func (f ObjectFunctions) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	if err := validateObjectID(objectID); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	return f.gateway.ReadFile(ctx, objectID, filename)
}

func (f ObjectFunctions) DeleteFile(ctx context.Context, objectID, filename string) (gateway.ManifestResult, error) {
	if err := validateObjectID(objectID); err != nil {
		return gateway.ManifestResult{}, model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	f.log.InfoContext(ctx, "object", "deleting object file", logging.String("object_id", objectID), logging.String("filename", filename))
	result, err := f.gateway.DeleteFile(ctx, objectID, filename)
	if err != nil {
		return gateway.ManifestResult{}, err
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

// PublishObjectUpdated publishes an object "updated" mutation after a successful
// outer mutation (e.g. streaming file write). Empty objectID returns INVALID_INPUT.
// Snapshot load failure still publishes ID-only and returns nil.
func (f ObjectFunctions) PublishObjectUpdated(ctx context.Context, objectID string) error {
	if objectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	f.publishObjectMutation(ctx, "updated", objectID)
	return nil
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
