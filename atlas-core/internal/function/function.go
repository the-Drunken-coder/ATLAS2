package function

import (
	"atlas.local/protocol"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/model"
	"github.com/anomalyco/atlas-core/internal/objectstorage"
	"github.com/anomalyco/atlas-core/internal/protocolvalidation"
	"github.com/anomalyco/atlas-core/internal/store"
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
}

func NewEntityFunctions(pgStore store.EntityStore, log *logging.Logger, protoValidator ProtocolValidator) EntityFunctions {
	return EntityFunctions{pgStore: pgStore, log: log, protoValidator: protoValidator}
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
	return f.pgStore.CreateEntity(ctx, entity)
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
	return f.pgStore.UpsertEntity(ctx, entity)
}

type ObjectFunctions struct {
	pgStore        store.ObjectStore
	objStore       store.ObjectStorageStore
	idemStore      store.IdempotencyStore
	log            *logging.Logger
	protoValidator ProtocolValidator
}

var errDecodeObjectManifest = errors.New("decode object manifest")

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

func NewObjectFunctions(pgStore store.ObjectStore, objStore store.ObjectStorageStore, idemStore store.IdempotencyStore, log *logging.Logger, protoValidator ProtocolValidator) ObjectFunctions {
	return ObjectFunctions{pgStore: pgStore, objStore: objStore, idemStore: idemStore, log: log, protoValidator: protoValidator}
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
		createFn := f.ensureObjectCreated
		if claimed {
			createFn = f.ensureObjectCreatedFresh
		}
		if err := createFn(ctx, obj); err != nil {
			return failClaimedIdempotency(ctx, f.idemStore, claimed, "object_create", idem.key, err)
		}
		return f.idemStore.MarkCompleted(ctx, "object_create", idem.key)
	}

	if issues := f.protoValidator.ValidateObject(obj); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	return f.createObjectInner(ctx, obj)
}

func (f ObjectFunctions) createObjectInner(ctx context.Context, obj *model.Object) error {
	f.log.InfoContext(ctx, "object", "creating object", logging.String("object_id", obj.ObjectID), logging.String("object_type", string(obj.Type)))
	return f.ensureObjectCreatedFresh(ctx, obj)
}

func (f ObjectFunctions) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	if objectID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	return f.pgStore.GetObject(ctx, objectID)
}

func (f ObjectFunctions) ListObjects(ctx context.Context, filters ...store.ObjectFilter) ([]model.Object, error) {
	return f.pgStore.ListObjects(ctx, filters...)
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
	return f.pgStore.UpdateObject(ctx, obj)
}

func (f ObjectFunctions) DeleteObject(ctx context.Context, objectID string) error {
	if objectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	obj, err := f.pgStore.GetObject(ctx, objectID)
	if err != nil {
		return err
	}
	f.log.InfoContext(ctx, "object", "deleting object", logging.String("object_id", objectID), logging.String("object_type", string(obj.Type)))
	if err := f.pgStore.DeleteObject(ctx, objectID); err != nil {
		return err
	}
	if err := f.objStore.DeleteObjectFolder(objectID); err != nil {
		if restoreErr := f.pgStore.UpsertObject(ctx, obj); restoreErr != nil {
			return errors.Join(model.NewCoreError("OBJECT_DELETE_ERROR", "failed to delete object storage and restore metadata"), err, restoreErr)
		}
		return err
	}
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
	_, existingErr := f.pgStore.GetObject(ctx, obj.ObjectID)
	objectExists := existingErr == nil
	if existingErr != nil && !errors.Is(existingErr, model.ErrNotFound) {
		return existingErr
	}
	f.log.InfoContext(ctx, "object", "upserting object", logging.String("object_id", obj.ObjectID), logging.String("object_type", string(obj.Type)))
	if err := f.pgStore.UpsertObject(ctx, obj); err != nil {
		return err
	}
	folderExists, err := f.objStore.ObjectFolderExists(obj.ObjectID)
	if err != nil {
		if !objectExists {
			if rollbackErr := f.pgStore.DeleteObject(ctx, obj.ObjectID); rollbackErr != nil {
				return errors.Join(model.NewCoreError("OBJECT_UPSERT_ERROR", "failed to inspect object storage and rollback metadata"), err, rollbackErr)
			}
		}
		return err
	}
	if !folderExists {
		if err := f.objStore.CreateObjectFolder(obj.ObjectID); err != nil {
			if rollbackErr := rollbackObjectUpsert(ctx, f.pgStore, f.objStore, obj.ObjectID, !objectExists); rollbackErr != nil {
				return errors.Join(model.NewCoreError("OBJECT_UPSERT_ERROR", "failed to initialize object storage"), err, rollbackErr)
			}
			return err
		}
	}
	f.syncObjectManifestFromFilesystemBestEffort(ctx, obj.ObjectID, "upsert")
	return nil
}

func (f ObjectFunctions) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	if objectID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	if _, err := f.pgStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	data, err := f.objStore.ReadManifestFile(objectID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, model.NewCoreError("MANIFEST_NOT_FOUND", fmt.Sprintf("manifest file is missing for object %s; object initialization should have created it", objectID))
		}
		return nil, err
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, model.NewCoreError("MANIFEST_ERROR", "failed to decode manifest: "+err.Error())
	}
	return model.NormalizeManifest(&manifest), nil
}

func (f ObjectFunctions) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest) error {
	if objectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	if manifest == nil {
		return model.NewFieldError("INVALID_INPUT", "manifest is required", "manifest")
	}
	if _, err := f.pgStore.GetObject(ctx, objectID); err != nil {
		return err
	}
	manifest = model.NormalizeManifest(manifest)
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return model.NewCoreError("MANIFEST_ERROR", "failed to marshal manifest: "+err.Error())
	}
	if err := f.objStore.WriteManifestFile(objectID, manifestBytes); err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := f.pgStore.UpdateObjectManifest(ctx, objectID, manifest, now); err != nil {
		return model.NewCoreError("MANIFEST_CACHE_SYNC_ERROR", "manifest written to filesystem but failed to update database cache: "+err.Error())
	}
	f.log.InfoContext(ctx, "object", "updated object manifest", logging.String("object_id", objectID), logging.String("manifest_version", manifest.Version))
	return nil
}

func (f ObjectFunctions) WriteFile(ctx context.Context, objectID, filename string, data []byte) error {
	if err := f.objStore.ValidateSafeObjectPath(objectID, filename); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := f.pgStore.GetObject(ctx, objectID); err != nil {
		return err
	}
	f.log.InfoContext(ctx, "object", "writing object file", logging.String("object_id", objectID), logging.String("filename", filename), logging.Any("size", len(data)))
	if err := f.objStore.WriteObjectFile(objectID, filename, data); err != nil {
		return err
	}
	return f.rebuildAndSyncObjectManifest(ctx, objectID)
}

func (f ObjectFunctions) AppendFile(ctx context.Context, objectID, filename string, data []byte) error {
	if err := f.objStore.ValidateSafeObjectPath(objectID, filename); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := f.pgStore.GetObject(ctx, objectID); err != nil {
		return err
	}
	f.log.InfoContext(ctx, "object", "appending object file", logging.String("object_id", objectID), logging.String("filename", filename), logging.Any("size", len(data)))
	if err := f.objStore.AppendObjectFile(objectID, filename, data); err != nil {
		return err
	}
	return f.rebuildAndSyncObjectManifest(ctx, objectID)
}

func (f ObjectFunctions) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	if err := f.objStore.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := f.pgStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	return f.objStore.ReadObjectFile(objectID, filename)
}

func (f ObjectFunctions) DeleteFile(ctx context.Context, objectID, filename string) error {
	if err := f.objStore.ValidateSafeObjectPath(objectID, filename); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := f.pgStore.GetObject(ctx, objectID); err != nil {
		return err
	}
	f.log.InfoContext(ctx, "object", "deleting object file", logging.String("object_id", objectID), logging.String("filename", filename))
	if err := f.objStore.DeleteObjectFile(objectID, filename); err != nil {
		return err
	}
	return f.rebuildAndSyncObjectManifest(ctx, objectID)
}

func (f ObjectFunctions) ListFiles(ctx context.Context, objectID string) ([]string, error) {
	if err := objectstorage.ValidateObjectID(objectID); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	if _, err := f.pgStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	return f.objStore.ListObjectFolderFiles(objectID)
}

func (f ObjectFunctions) Reconcile(ctx context.Context) error {
	f.log.InfoContext(ctx, "object_reconcile", "starting object reconciliation")
	objects, err := f.pgStore.ListObjects(ctx)
	if err != nil {
		return fmt.Errorf("list database objects: %w", err)
	}
	folders, err := f.objStore.ListObjectFolders()
	if err != nil {
		return fmt.Errorf("list object folders: %w", err)
	}

	dbObjects := make(map[string]model.Object, len(objects))
	for _, obj := range objects {
		dbObjects[obj.ObjectID] = obj
	}
	for _, folder := range folders {
		if _, ok := dbObjects[folder]; !ok {
			if err := f.restoreOrphanObjectFromFilesystem(ctx, folder); err != nil {
				if !errors.Is(err, model.ErrNotFound) {
					return fmt.Errorf("restore orphan object folder %s: %w", folder, err)
				}
				// Re-check database to handle race condition: object may have been created
				// between initial scan and this restore attempt
				if _, dbErr := f.pgStore.GetObject(ctx, folder); dbErr == nil {
					// Object now exists in database, skip deletion
					f.log.InfoContext(ctx, "object_reconcile", "object created during reconciliation, skipping folder deletion", logging.String("object_id", folder))
					continue
				} else if !errors.Is(dbErr, model.ErrNotFound) {
					return fmt.Errorf("re-check object existence for %s: %w", folder, dbErr)
				}
				// Object confirmed non-existent in database, safe to delete folder
				f.log.WarnContext(ctx, "object_reconcile", "removing orphan object folder without manifest", logging.String("object_id", folder))
				if err := f.objStore.DeleteObjectFolder(folder); err != nil {
					return fmt.Errorf("delete orphan object folder %s: %w", folder, err)
				}
			}
			continue
		}
		if err := f.syncObjectManifestFromFilesystemWithRepair(ctx, folder); err != nil {
			return fmt.Errorf("sync object manifest %s: %w", folder, err)
		}
		delete(dbObjects, folder)
	}

	remaining := make([]string, 0, len(dbObjects))
	for objectID := range dbObjects {
		remaining = append(remaining, objectID)
	}
	sort.Strings(remaining)
	for _, objectID := range remaining {
		f.log.WarnContext(ctx, "object_reconcile", "recreating missing object folder from metadata", logging.String("object_id", objectID))
		if err := f.objStore.CreateObjectFolder(objectID); err != nil {
			return fmt.Errorf("create missing object folder %s: %w", objectID, err)
		}
		if err := f.syncObjectManifestFromFilesystemWithRepair(ctx, objectID); err != nil {
			return fmt.Errorf("sync recreated object manifest %s: %w", objectID, err)
		}
	}

	f.log.InfoContext(ctx, "object_reconcile", "finished object reconciliation")
	return nil
}

func (f ObjectFunctions) syncObjectManifestFromFilesystem(ctx context.Context, objectID string) error {
	data, err := f.objStore.ReadManifestFile(objectID)
	if err != nil {
		return err
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("%w for %s: %w", errDecodeObjectManifest, objectID, err)
	}
	manifestPtr := model.NormalizeManifest(&manifest)
	cachedManifest, err := f.pgStore.GetObjectManifest(ctx, objectID)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return err
	}
	if cachedManifest != nil && cachedManifest.Version == manifestPtr.Version {
		return nil
	}
	return f.pgStore.UpdateObjectManifest(ctx, objectID, manifestPtr, time.Now().UTC())
}

func (f ObjectFunctions) syncObjectManifestFromFilesystemWithRepair(ctx context.Context, objectID string) error {
	err := f.syncObjectManifestFromFilesystem(ctx, objectID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, model.ErrNotFound) && !errors.Is(err, errDecodeObjectManifest) {
		return err
	}
	f.log.WarnContext(ctx, "object_reconcile", "repairing object manifest during reconciliation",
		logging.String("object_id", objectID),
		logging.ErrorField(err),
	)
	if repairErr := f.repairObjectManifestFile(objectID); repairErr != nil {
		return errors.Join(err, repairErr)
	}
	return f.syncObjectManifestFromFilesystem(ctx, objectID)
}

func (f ObjectFunctions) restoreOrphanObjectFromFilesystem(ctx context.Context, objectID string) error {
	if err := objectstorage.ValidateObjectID(objectID); err != nil {
		return err
	}

	// Check if object metadata exists in the database FIRST before attempting restore.
	// This handles race conditions where the object may have been created between
	// the reconciliation scan and this restore attempt.
	existingObj, err := f.pgStore.GetObject(ctx, objectID)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("check existing object metadata: %w", err)
	}

	manifest, err := f.readObjectManifestFromFilesystem(objectID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			// Manifest not found - only return ErrNotFound if object also doesn't exist in DB
			if existingObj == nil {
				return err
			}
			// Object exists in DB but manifest missing - will be repaired below
		} else if !errors.Is(err, errDecodeObjectManifest) {
			return err
		}
		// Try to repair the manifest
		if err := f.repairObjectManifestFile(objectID); err != nil {
			return err
		}
		manifest, err = f.readObjectManifestFromFilesystem(objectID)
		if err != nil {
			return err
		}
	}

	now := time.Now().UTC()

	// If object metadata exists in database, use it; otherwise we cannot safely
	// restore from the filesystem manifest alone, so recreate the row with the
	// safest default metadata we have for orphaned local objects and then sync
	// the manifest cache from the authoritative filesystem copy.
	if existingObj != nil {
		// Object exists in database but manifest is out of sync - just update manifest
		f.log.WarnContext(ctx, "object_reconcile", "syncing manifest for existing object",
			logging.String("object_id", objectID),
			logging.String("manifest_version", manifest.Version),
		)
		return f.pgStore.UpdateObjectManifest(ctx, objectID, manifest, now)
	}

	restored := &model.Object{
		ObjectID: objectID,
		// The manifest only proves that an object folder exists, so restore with
		// the least-privileged generic metadata we have and let later workflows
		// reclassify it if richer ownership/type information becomes available.
		Type:      model.ObjectTypeLog,
		OwnerType: model.OwnerTypeSystem,
		OwnerID:   "system",
		JSON:      []byte("{}"),
		CreatedAt: now,
		UpdatedAt: now,
	}
	f.log.WarnContext(ctx, "object_reconcile", "restoring orphan object metadata from filesystem manifest",
		logging.String("object_id", objectID),
		logging.String("manifest_version", manifest.Version),
	)
	if err := f.pgStore.CreateObject(ctx, restored); err != nil {
		if !errors.Is(err, model.ErrConflict) {
			return fmt.Errorf("create restored object metadata: %w", err)
		}
	}
	return f.pgStore.UpdateObjectManifest(ctx, objectID, manifest, now)
}

func (f ObjectFunctions) syncObjectManifestFromFilesystemBestEffort(ctx context.Context, objectID, operation string) {
	if err := f.syncObjectManifestFromFilesystem(ctx, objectID); err != nil {
		f.log.WarnContext(ctx, "object", "object write succeeded but manifest cache refresh failed",
			logging.String("object_id", objectID),
			logging.String("operation", operation),
			logging.ErrorField(err),
		)
	}
}

func (f ObjectFunctions) rebuildAndSyncObjectManifest(ctx context.Context, objectID string) error {
	manifest, err := f.rebuildObjectManifestFromFilesystem(objectID)
	if err != nil {
		return err
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal rebuilt manifest for %s: %w", objectID, err)
	}
	if err := f.objStore.WriteManifestFile(objectID, manifestData); err != nil {
		return fmt.Errorf("rewrite manifest for %s: %w", objectID, err)
	}
	return f.pgStore.UpdateObjectManifest(ctx, objectID, manifest, time.Now().UTC())
}

func (f ObjectFunctions) ensureObjectCreatedFresh(ctx context.Context, obj *model.Object) error {
	if err := f.pgStore.CreateObject(ctx, obj); err != nil {
		return err
	}
	if err := f.ensureObjectFolderReady(obj.ObjectID); err != nil {
		if rollbackErr := rollbackObjectCreate(ctx, f.pgStore, f.objStore, obj.ObjectID); rollbackErr != nil {
			return errors.Join(model.NewCoreError("OBJECT_CREATE_ERROR", "failed to initialize object storage"), err, rollbackErr)
		}
		return err
	}
	f.syncObjectManifestFromFilesystemBestEffort(ctx, obj.ObjectID, "create")
	return nil
}

func (f ObjectFunctions) ensureObjectCreated(ctx context.Context, obj *model.Object) error {
	metadataCreated := false
	if _, err := f.pgStore.GetObject(ctx, obj.ObjectID); err != nil {
		if !errors.Is(err, model.ErrNotFound) {
			return err
		}
		if err := f.pgStore.CreateObject(ctx, obj); err != nil {
			if !errors.Is(err, model.ErrConflict) {
				return err
			}
		} else {
			metadataCreated = true
		}
	}
	if err := f.ensureObjectFolderReady(obj.ObjectID); err != nil {
		if metadataCreated {
			if rollbackErr := rollbackObjectCreate(ctx, f.pgStore, f.objStore, obj.ObjectID); rollbackErr != nil {
				return errors.Join(model.NewCoreError("OBJECT_CREATE_ERROR", "failed to recover object storage"), err, rollbackErr)
			}
		}
		return err
	}
	f.syncObjectManifestFromFilesystemBestEffort(ctx, obj.ObjectID, "create")
	return nil
}

func (f ObjectFunctions) ensureObjectFolderReady(objectID string) error {
	err := f.objStore.CreateObjectFolder(objectID)
	if err == nil {
		return nil
	}
	exists, existsErr := f.objStore.ObjectFolderExists(objectID)
	if existsErr != nil {
		return errors.Join(err, existsErr)
	}
	if !exists {
		return err
	}
	if repairErr := f.repairObjectManifestFile(objectID); repairErr != nil {
		return errors.Join(err, repairErr)
	}
	return nil
}

func (f ObjectFunctions) repairObjectManifestFile(objectID string) error {
	manifest, err := f.rebuildObjectManifestFromFilesystem(objectID)
	if err != nil {
		return err
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal rebuilt manifest for %s: %w", objectID, err)
	}
	if err := f.objStore.WriteManifestFile(objectID, manifestData); err != nil {
		return fmt.Errorf("rewrite manifest for %s: %w", objectID, err)
	}
	return nil
}

func (f ObjectFunctions) readObjectManifestFromFilesystem(objectID string) (*model.ObjectManifest, error) {
	data, err := f.objStore.ReadManifestFile(objectID)
	if err != nil {
		return nil, err
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%w for %s: %w", errDecodeObjectManifest, objectID, err)
	}
	return model.NormalizeManifest(&manifest), nil
}

func (f ObjectFunctions) rebuildObjectManifestFromFilesystem(objectID string) (*model.ObjectManifest, error) {
	files, err := f.objStore.ListObjectFolderFiles(objectID)
	if err != nil {
		return nil, fmt.Errorf("list object files for %s: %w", objectID, err)
	}
	manifest := &model.ObjectManifest{Files: make(map[string]model.ObjectFileInfo, len(files))}
	for _, name := range files {
		info, err := f.objStore.GetObjectFileInfo(objectID, name)
		if err != nil {
			return nil, fmt.Errorf("stat object file %s/%s: %w", objectID, name, err)
		}
		manifest.Files[name] = info
	}
	return model.NormalizeManifest(manifest), nil
}

type TaskFunctions struct {
	taskStore      store.TaskStore
	objectStore    store.ObjectStore
	entityStore    store.EntityStore
	idemStore      store.IdempotencyStore
	log            *logging.Logger
	protoValidator ProtocolValidator
}

func NewTaskFunctions(taskStore store.TaskStore, objectStore store.ObjectStore, entityStore store.EntityStore, idemStore store.IdempotencyStore, log *logging.Logger, protoValidator ProtocolValidator) TaskFunctions {
	return TaskFunctions{taskStore: taskStore, objectStore: objectStore, entityStore: entityStore, idemStore: idemStore, log: log, protoValidator: protoValidator}
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
		return f.idemStore.MarkCompleted(ctx, "task_create", idem.key)
	}

	if err := f.validateTaskRuntime(ctx, task); err != nil {
		return err
	}
	return f.createTaskInner(ctx, task)
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
	return f.taskStore.UpsertTask(ctx, task)
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
	// Protocol validation already guarantees valid JSON structure; unmarshal
	// failures or missing fields fall through to the catalog lookup below.
	_ = json.Unmarshal(task.JSON, &taskJSON)
	commandType := taskJSON.Components.Command.Type

	var catalogJSON map[string]any
	_ = json.Unmarshal(catalogObj.JSON, &catalogJSON)
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
}

func NewObservationFunctions(pgStore store.ObservationStore, log *logging.Logger, protoValidator ProtocolValidator) ObservationFunctions {
	return ObservationFunctions{pgStore: pgStore, log: log, protoValidator: protoValidator}
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
	return f.pgStore.CreateObservation(ctx, obs)
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
	return f.pgStore.UpsertObservation(ctx, obs)
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
	if err := objectstorage.ValidateObjectID(obj.ObjectID); err != nil {
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

func rollbackObjectCreate(ctx context.Context, pgStore store.ObjectStore, objStore store.ObjectStorageStore, objectID string) error {
	var failures []string
	if err := objStore.DeleteObjectFolder(objectID); err != nil {
		failures = append(failures, "cleanup partial object folder failed: "+err.Error())
	}
	if err := pgStore.DeleteObject(ctx, objectID); err != nil {
		failures = append(failures, "rollback metadata failed: "+err.Error())
	}
	if len(failures) == 0 {
		return nil
	}
	return model.NewCoreError("OBJECT_CREATE_ROLLBACK_ERROR", strings.Join(failures, "; "))
}

func rollbackObjectUpsert(ctx context.Context, pgStore store.ObjectStore, objStore store.ObjectStorageStore, objectID string, rollbackMetadata bool) error {
	var failures []string
	if err := objStore.DeleteObjectFolder(objectID); err != nil {
		failures = append(failures, "cleanup partial object folder failed: "+err.Error())
	}
	if rollbackMetadata {
		if err := pgStore.DeleteObject(ctx, objectID); err != nil {
			failures = append(failures, "rollback metadata failed: "+err.Error())
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return model.NewCoreError("OBJECT_UPSERT_ROLLBACK_ERROR", strings.Join(failures, "; "))
}

func isKnownObjectType(objectType model.ObjectType) bool {
	for _, known := range model.KnownObjectTypes() {
		if known == objectType {
			return true
		}
	}
	return false
}
