package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/anomalyco/atlas-core/internal/adapters/objectstorage"
	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

var errDecodeObjectManifest = errors.New("decode object manifest")

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
