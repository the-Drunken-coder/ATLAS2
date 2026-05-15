package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/datastorage/internal/objectstorage"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

var errDecodeObjectManifest = errors.New("decode object manifest")

func (s *Service) CreateObject(ctx context.Context, object *model.Object) error {
	s.Logger.InfoContext(ctx, "object", "creating object", logging.String("object_id", object.ObjectID), logging.String("object_type", string(object.Type)))
	return s.createObjectFresh(ctx, object)
}

func (s *Service) EnsureObjectCreated(ctx context.Context, object *model.Object) error {
	metadataCreated := false
	if _, err := s.objectStore.GetObject(ctx, object.ObjectID); err != nil {
		if !errors.Is(err, model.ErrNotFound) {
			return err
		}
		if err := s.objectStore.CreateObject(ctx, object); err != nil {
			if !errors.Is(err, model.ErrConflict) {
				return err
			}
		} else {
			metadataCreated = true
		}
	}
	if err := s.ensureObjectFolderReady(object.ObjectID); err != nil {
		if metadataCreated {
			if rollbackErr := rollbackObjectCreate(ctx, s.objectStore, s.objectStorage, object.ObjectID); rollbackErr != nil {
				return errors.Join(model.NewCoreError("OBJECT_CREATE_ERROR", "failed to recover object storage"), err, rollbackErr)
			}
		}
		return err
	}
	s.syncObjectManifestFromFilesystemBestEffort(ctx, object.ObjectID, "create")
	stored, err := s.objectStore.GetObject(ctx, object.ObjectID)
	if err == nil {
		*object = *stored
	}
	return err
}

func (s *Service) createObjectFresh(ctx context.Context, object *model.Object) error {
	if err := s.objectStore.CreateObject(ctx, object); err != nil {
		return err
	}
	if err := s.ensureObjectFolderReady(object.ObjectID); err != nil {
		if rollbackErr := rollbackObjectCreate(ctx, s.objectStore, s.objectStorage, object.ObjectID); rollbackErr != nil {
			return errors.Join(model.NewCoreError("OBJECT_CREATE_ERROR", "failed to initialize object storage"), err, rollbackErr)
		}
		return err
	}
	s.syncObjectManifestFromFilesystemBestEffort(ctx, object.ObjectID, "create")
	stored, err := s.objectStore.GetObject(ctx, object.ObjectID)
	if err == nil {
		*object = *stored
	}
	return err
}

func (s *Service) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	return s.objectStore.GetObject(ctx, objectID)
}

func (s *Service) ListObjects(ctx context.Context, filters ...store.ObjectFilter) ([]model.Object, error) {
	return s.objectStore.ListObjects(ctx, filters...)
}

func (s *Service) UpdateObject(ctx context.Context, object *model.Object) error {
	return s.objectStore.UpdateObject(ctx, object)
}

func (s *Service) DeleteObject(ctx context.Context, objectID string) error {
	object, err := s.objectStore.GetObject(ctx, objectID)
	if err != nil {
		return err
	}
	if err := s.objectStore.DeleteObject(ctx, objectID); err != nil {
		return err
	}
	if err := s.objectStorage.DeleteObjectFolder(objectID); err != nil {
		if restoreErr := s.objectStore.UpsertObject(ctx, object); restoreErr != nil {
			return errors.Join(model.NewCoreError("OBJECT_DELETE_ERROR", "failed to delete object storage and restore metadata"), err, restoreErr)
		}
		return err
	}
	return nil
}

func (s *Service) UpsertObject(ctx context.Context, object *model.Object) error {
	_, existingErr := s.objectStore.GetObject(ctx, object.ObjectID)
	objectExists := existingErr == nil
	if existingErr != nil && !errors.Is(existingErr, model.ErrNotFound) {
		return existingErr
	}
	if err := s.objectStore.UpsertObject(ctx, object); err != nil {
		return err
	}
	folderExists, err := s.objectStorage.ObjectFolderExists(object.ObjectID)
	if err != nil {
		if !objectExists {
			if rollbackErr := s.objectStore.DeleteObject(ctx, object.ObjectID); rollbackErr != nil {
				return errors.Join(model.NewCoreError("OBJECT_UPSERT_ERROR", "failed to inspect object storage and rollback metadata"), err, rollbackErr)
			}
		}
		return err
	}
	if !folderExists {
		if err := s.objectStorage.CreateObjectFolder(object.ObjectID); err != nil {
			if rollbackErr := rollbackObjectUpsert(ctx, s.objectStore, s.objectStorage, object.ObjectID, !objectExists); rollbackErr != nil {
				return errors.Join(model.NewCoreError("OBJECT_UPSERT_ERROR", "failed to initialize object storage"), err, rollbackErr)
			}
			return err
		}
	}
	s.syncObjectManifestFromFilesystemBestEffort(ctx, object.ObjectID, "upsert")
	stored, err := s.objectStore.GetObject(ctx, object.ObjectID)
	if err == nil {
		*object = *stored
	}
	return err
}

func (s *Service) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	data, err := s.objectStorage.ReadManifestFile(objectID)
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

func (s *Service) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest) (*model.ObjectManifest, error) {
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	manifest = model.NormalizeManifest(manifest)
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, model.NewCoreError("MANIFEST_ERROR", "failed to marshal manifest: "+err.Error())
	}
	if err := s.objectStorage.WriteManifestFile(objectID, manifestBytes); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if err := s.objectStore.UpdateObjectManifest(ctx, objectID, manifest, now); err != nil {
		return nil, model.NewCoreError("MANIFEST_CACHE_SYNC_ERROR", "manifest written to filesystem but failed to update database cache: "+err.Error())
	}
	return manifest, nil
}

func (s *Service) WriteObjectFile(ctx context.Context, objectID, filename string, data []byte) (*model.ObjectManifest, error) {
	if err := s.objectStorage.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	if err := s.objectStorage.WriteObjectFile(objectID, filename, data); err != nil {
		return nil, err
	}
	return s.bestEffortSyncManifest(ctx, objectID, "WriteObjectFile")
}

func (s *Service) AppendObjectFile(ctx context.Context, objectID, filename string, data []byte) (*model.ObjectManifest, error) {
	if err := s.objectStorage.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	if err := s.objectStorage.AppendObjectFile(objectID, filename, data); err != nil {
		return nil, err
	}
	return s.bestEffortSyncManifest(ctx, objectID, "AppendObjectFile")
}

func (s *Service) ReadObjectFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	if err := s.objectStorage.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	return s.objectStorage.ReadObjectFile(objectID, filename)
}

func (s *Service) DeleteObjectFile(ctx context.Context, objectID, filename string) (*model.ObjectManifest, error) {
	if err := s.objectStorage.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	if err := s.objectStorage.DeleteObjectFile(objectID, filename); err != nil {
		return nil, err
	}
	return s.bestEffortSyncManifest(ctx, objectID, "DeleteObjectFile")
}

func (s *Service) ListObjectFiles(ctx context.Context, objectID string) ([]string, error) {
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	return s.objectStorage.ListObjectFolderFiles(objectID)
}

func (s *Service) ReconcileObjects(ctx context.Context) error {
	s.Logger.InfoContext(ctx, "object_reconcile", "starting object reconciliation")
	objects, err := s.objectStore.ListObjects(ctx)
	if err != nil {
		return fmt.Errorf("list database objects: %w", err)
	}
	folders, err := s.objectStorage.ListObjectFolders()
	if err != nil {
		return fmt.Errorf("list object folders: %w", err)
	}

	dbObjects := make(map[string]model.Object, len(objects))
	for _, object := range objects {
		dbObjects[object.ObjectID] = object
	}
	folderSet := make(map[string]struct{}, len(folders))
	for _, folder := range folders {
		folderSet[folder] = struct{}{}
	}
	for _, folder := range folders {
		if err := objectstorage.ValidateObjectID(folder); err != nil {
			s.Logger.WarnContext(ctx, "object_reconcile", "deleting invalid object folder", logging.String("object_id", folder), logging.ErrorField(err))
			if deleteErr := s.objectStorage.DeleteObjectFolder(folder); deleteErr != nil {
				return fmt.Errorf("delete invalid object folder %s: %w", folder, deleteErr)
			}
			continue
		}
		if _, ok := dbObjects[folder]; !ok {
			s.quarantineOrphanFolder(ctx, folder)
			continue
		}
		if err := s.syncObjectManifestFromFilesystemWithRepair(ctx, folder); err != nil {
			s.Logger.WarnContext(ctx, "object_reconcile", "manifest repair failed", logging.String("object_id", folder), logging.ErrorField(err))
		}
	}

	for _, object := range objects {
		if _, ok := folderSet[object.ObjectID]; !ok {
			s.Logger.DebugContext(ctx, "object_reconcile", "database object has no filesystem folder", logging.String("object_id", object.ObjectID))
		}
	}

	s.Logger.InfoContext(ctx, "object_reconcile", "finished object reconciliation")
	return nil
}

func (s *Service) quarantineOrphanFolder(ctx context.Context, folder string) {
	timestamp := time.Now().Unix()
	quarantineName := fmt.Sprintf(".quarantine-%s-%d", folder, timestamp)
	s.Logger.WarnContext(ctx, "object_reconcile", "quarantining orphan folder (no DB row)", logging.String("folder", folder), logging.String("quarantine_name", quarantineName))
	if err := s.objectStorage.RenameObjectFolder(folder, quarantineName); err != nil {
		s.Logger.WarnContext(ctx, "object_reconcile", "quarantine rename failed, deleting orphan folder", logging.String("folder", folder), logging.ErrorField(err))
		if deleteErr := s.objectStorage.DeleteObjectFolder(folder); deleteErr != nil {
			s.Logger.ErrorContext(ctx, "object_reconcile", "failed to delete orphan folder", logging.String("folder", folder), logging.ErrorField(deleteErr))
		}
	}
}

func (s *Service) syncObjectManifestFromFilesystem(ctx context.Context, objectID string) error {
	data, err := s.objectStorage.ReadManifestFile(objectID)
	if err != nil {
		return err
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("%w for %s: %w", errDecodeObjectManifest, objectID, err)
	}
	manifestPtr := model.NormalizeManifest(&manifest)
	cachedManifest, err := s.objectStore.GetObjectManifest(ctx, objectID)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return err
	}
	if cachedManifest != nil && cachedManifest.Version == manifestPtr.Version {
		return nil
	}
	return s.objectStore.UpdateObjectManifest(ctx, objectID, manifestPtr, time.Now().UTC())
}

func (s *Service) syncObjectManifestFromFilesystemWithRepair(ctx context.Context, objectID string) error {
	err := s.syncObjectManifestFromFilesystem(ctx, objectID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, model.ErrNotFound) && !errors.Is(err, errDecodeObjectManifest) {
		return err
	}
	if repairErr := s.repairObjectManifestFile(objectID); repairErr != nil {
		return errors.Join(err, repairErr)
	}
	return s.syncObjectManifestFromFilesystem(ctx, objectID)
}

func (s *Service) syncObjectManifestFromFilesystemBestEffort(ctx context.Context, objectID, operation string) {
	if err := s.syncObjectManifestFromFilesystem(ctx, objectID); err != nil {
		s.Logger.WarnContext(ctx, "object", "object write succeeded but manifest cache refresh failed", logging.String("object_id", objectID), logging.String("operation", operation), logging.ErrorField(err))
	}
}

func (s *Service) rebuildAndSyncObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	manifest, err := s.rebuildObjectManifestFromFilesystem(objectID)
	if err != nil {
		return nil, err
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal rebuilt manifest for %s: %w", objectID, err)
	}
	if err := s.objectStorage.WriteManifestFile(objectID, manifestData); err != nil {
		return nil, fmt.Errorf("rewrite manifest for %s: %w", objectID, err)
	}
	if err := s.objectStore.UpdateObjectManifest(ctx, objectID, manifest, time.Now().UTC()); err != nil {
		return nil, err
	}
	return manifest, nil
}

// bestEffortSyncManifest calls rebuildAndSyncObjectManifest and logs a warning
// on failure instead of returning an error. The file mutation already committed;
// a failed manifest sync is repaired by the next reconcile pass, so returning an
// error would be misleading and make the operation non-idempotent.
func (s *Service) bestEffortSyncManifest(ctx context.Context, objectID, caller string) (*model.ObjectManifest, error) {
	manifest, err := s.rebuildAndSyncObjectManifest(ctx, objectID)
	if err != nil {
		s.Logger.WarnContext(ctx, "object", "manifest sync after mutation failed (reconcile will repair)",
			logging.String("object_id", objectID),
			logging.String("caller", caller),
			logging.ErrorField(err),
		)
		return nil, nil
	}
	return manifest, nil
}

func (s *Service) ensureObjectFolderReady(objectID string) error {
	err := s.objectStorage.CreateObjectFolder(objectID)
	if err == nil {
		return nil
	}
	exists, existsErr := s.objectStorage.ObjectFolderExists(objectID)
	if existsErr != nil {
		return errors.Join(err, existsErr)
	}
	if !exists {
		return err
	}
	if repairErr := s.repairObjectManifestFile(objectID); repairErr != nil {
		return errors.Join(err, repairErr)
	}
	return nil
}

func (s *Service) repairObjectManifestFile(objectID string) error {
	manifest, err := s.rebuildObjectManifestFromFilesystem(objectID)
	if err != nil {
		return err
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal rebuilt manifest for %s: %w", objectID, err)
	}
	if err := s.objectStorage.WriteManifestFile(objectID, manifestData); err != nil {
		return fmt.Errorf("rewrite manifest for %s: %w", objectID, err)
	}
	return nil
}

func (s *Service) rebuildObjectManifestFromFilesystem(objectID string) (*model.ObjectManifest, error) {
	files, err := s.objectStorage.ListObjectFolderFiles(objectID)
	if err != nil {
		return nil, fmt.Errorf("list object files for %s: %w", objectID, err)
	}
	manifest := &model.ObjectManifest{Files: make(map[string]model.ObjectFileInfo, len(files))}
	for _, name := range files {
		info, err := s.objectStorage.GetObjectFileInfo(objectID, name)
		if err != nil {
			return nil, fmt.Errorf("stat object file %s/%s: %w", objectID, name, err)
		}
		manifest.Files[name] = info
	}
	return model.NormalizeManifest(manifest), nil
}

func rollbackObjectCreate(ctx context.Context, objectStore interface {
	DeleteObject(context.Context, string) error
}, objStore interface{ DeleteObjectFolder(string) error }, objectID string) error {
	var failures []string
	if err := objStore.DeleteObjectFolder(objectID); err != nil {
		failures = append(failures, "cleanup partial object folder failed: "+err.Error())
	}
	if err := objectStore.DeleteObject(ctx, objectID); err != nil {
		failures = append(failures, "rollback metadata failed: "+err.Error())
	}
	if len(failures) == 0 {
		return nil
	}
	return model.NewCoreError("OBJECT_CREATE_ROLLBACK_ERROR", failures[0])
}

func rollbackObjectUpsert(ctx context.Context, objectStore interface {
	DeleteObject(context.Context, string) error
}, objStore interface{ DeleteObjectFolder(string) error }, objectID string, rollbackMetadata bool) error {
	var failures []string
	if err := objStore.DeleteObjectFolder(objectID); err != nil {
		failures = append(failures, "cleanup partial object folder failed: "+err.Error())
	}
	if rollbackMetadata {
		if err := objectStore.DeleteObject(ctx, objectID); err != nil {
			failures = append(failures, "rollback metadata failed: "+err.Error())
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return model.NewCoreError("OBJECT_UPSERT_ROLLBACK_ERROR", failures[0])
}
