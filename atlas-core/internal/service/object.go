package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
	"github.com/anomalyco/atlas-core/internal/validation/blob"
	manifestval "github.com/anomalyco/atlas-core/internal/validation/manifest"
)

type ObjectFunctions struct {
	pgStore   ports.ObjectStore
	objStore  ports.ObjectStorageStore
	idemStore ports.IdempotencyStore
	log       *logging.Logger
}

func (f ObjectFunctions) CreateObject(ctx context.Context, obj *model.Object, opts ...IdempotencyOption) error {
	if err := validateObjectModel(obj); err != nil {
		return err
	}
	if err := blob.NormalizeObject(obj, blob.OperationCreate); err != nil {
		return err
	}
	now := time.Now().UTC()
	if obj.CreatedAt.IsZero() {
		obj.CreatedAt = now
	}
	if obj.UpdatedAt.IsZero() {
		obj.UpdatedAt = now
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
			if record.Status == ports.IdempotencyStatusCompleted {
				f.log.InfoContext(ctx, "object", "idempotent create replay",
					logging.String("object_id", obj.ObjectID),
					logging.String("idempotency_key", idem.key),
				)
				return nil
			}
		}
		createFn := f.ensureObjectCreated
		if claimed {
			createFn = f.ensureObjectCreatedFresh
		}
		if err := createFn(ctx, obj); err != nil {
			if claimed {
				if markErr := f.idemStore.MarkFailed(ctx, "object_create", idem.key); markErr != nil {
					return errors.Join(err, markErr)
				}
			}
			return err
		}
		return f.idemStore.MarkCompleted(ctx, "object_create", idem.key)
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

func (f ObjectFunctions) ListObjects(ctx context.Context, filters ...ports.ObjectFilter) ([]model.Object, error) {
	return f.pgStore.ListObjects(ctx, filters...)
}

func (f ObjectFunctions) UpdateObject(ctx context.Context, obj *model.Object) error {
	if err := validateObjectModel(obj); err != nil {
		return err
	}
	if err := blob.NormalizeObject(obj, blob.OperationUpdate); err != nil {
		return err
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
	if err := blob.NormalizeObject(obj, blob.OperationUpsert); err != nil {
		return err
	}
	now := time.Now().UTC()
	if obj.CreatedAt.IsZero() {
		obj.CreatedAt = now
	}
	obj.UpdatedAt = now
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
	if err := manifestval.ValidateObjectManifest(manifest); err != nil {
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

func rollbackObjectCreate(ctx context.Context, pgStore ports.ObjectStore, objStore ports.ObjectStorageStore, objectID string) error {
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

func rollbackObjectUpsert(ctx context.Context, pgStore ports.ObjectStore, objStore ports.ObjectStorageStore, objectID string, rollbackMetadata bool) error {
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
