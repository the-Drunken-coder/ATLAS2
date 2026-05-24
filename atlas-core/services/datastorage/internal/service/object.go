package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/listcursor"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/objectpath"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

var errDecodeObjectManifest = errors.New("decode object manifest")

var quarantineTimestampFunc = func() int64 { return time.Now().UnixNano() }

const quarantineFolderPrefix = "quarantine-"

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
	s.ensureObjectManifestFileBestEffort(ctx, object.ObjectID, "create")
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
	s.ensureObjectManifestFileBestEffort(ctx, object.ObjectID, "create")
	stored, err := s.objectStore.GetObject(ctx, object.ObjectID)
	if err == nil {
		*object = *stored
	}
	return err
}

func (s *Service) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	return s.objectStore.GetObject(ctx, objectID)
}

func (s *Service) ListObjects(ctx context.Context, params store.ObjectListParams) (store.ObjectListResult, error) {
	return s.objectStore.ListObjects(ctx, params)
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
	s.ensureObjectManifestFileBestEffort(ctx, object.ObjectID, "upsert")
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
	return manifest, nil
}

func (s *Service) WriteObjectFile(ctx context.Context, objectID, filename string, data []byte) (*model.ObjectManifest, error) {
	return s.StreamWriteObjectFile(ctx, objectID, filename, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func (s *Service) StreamWriteObjectFile(ctx context.Context, objectID, filename string, write func(io.Writer) error) (*model.ObjectManifest, error) {
	if err := s.objectStorage.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	if err := s.objectStorage.StreamWriteObjectFile(objectID, filename, write); err != nil {
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

func (s *Service) StreamAppendObjectFile(
	ctx context.Context,
	objectID, filename string,
	currentExpectedSize int64,
	write func(io.Writer, int64) error,
) (*model.ObjectManifest, error) {
	if err := s.objectStorage.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	if err := s.objectStorage.StreamAppendObjectFile(objectID, filename, currentExpectedSize, write); err != nil {
		return nil, err
	}
	return s.bestEffortSyncManifest(ctx, objectID, "AppendObjectFile")
}

func (s *Service) OpenReadObjectFile(ctx context.Context, objectID, filename string) (io.ReadCloser, int64, error) {
	if err := s.objectStorage.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, 0, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := s.objectStore.GetObject(ctx, objectID); err != nil {
		return nil, 0, err
	}
	info, err := s.objectStorage.GetObjectFileInfo(objectID, filename)
	if err != nil {
		return nil, 0, err
	}
	reader, err := s.objectStorage.ReaderForObjectFile(objectID, filename)
	if err != nil {
		return nil, 0, err
	}
	return reader, info.Size, nil
}

func (s *Service) ReadObjectFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	reader, _, err := s.OpenReadObjectFile(ctx, objectID, filename)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
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
	objects, err := s.listAllObjectsForReconcile(ctx)
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
		if strings.HasPrefix(folder, quarantineFolderPrefix) {
			continue
		}
		if err := objectpath.ValidateDeletableFolderName(folder); err != nil {
			s.Logger.WarnContext(ctx, "object_reconcile", "deleting invalid object folder", logging.String("object_id", folder), logging.ErrorField(err))
			if deleteErr := s.objectStorage.DeleteInvalidObjectFolder(folder); deleteErr != nil {
				return fmt.Errorf("delete invalid object folder %s: %w", folder, deleteErr)
			}
			continue
		}
		if _, ok := dbObjects[folder]; !ok {
			s.quarantineOrphanFolder(ctx, folder)
			continue
		}
		if err := s.repairObjectManifestFileIfNeeded(folder); err != nil {
			s.Logger.WarnContext(ctx, "object_reconcile", "manifest repair failed", logging.String("object_id", folder), logging.ErrorField(err))
		}
	}

	for _, object := range objects {
		if _, ok := folderSet[object.ObjectID]; !ok {
			s.Logger.WarnContext(ctx, "object_reconcile", "repairing missing object folder for existing DB row", logging.String("object_id", object.ObjectID))
			if err := s.objectStorage.CreateObjectFolder(object.ObjectID); err != nil {
				s.Logger.ErrorContext(ctx, "object_reconcile", "failed to create missing object folder", logging.String("object_id", object.ObjectID), logging.ErrorField(err))
				continue
			}
			if _, err := s.repairObjectManifestFile(object.ObjectID); err != nil {
				s.Logger.WarnContext(ctx, "object_reconcile", "failed to build manifest for recreated folder", logging.String("object_id", object.ObjectID), logging.ErrorField(err))
			}
		}
	}

	s.Logger.InfoContext(ctx, "object_reconcile", "finished object reconciliation")
	return nil
}

func (s *Service) listAllObjectsForReconcile(ctx context.Context) ([]model.Object, error) {
	var objects []model.Object
	pageToken := ""
	for {
		listRes, err := s.objectStore.ListObjects(ctx, store.ObjectListParams{
			PageSize:  int32(listcursor.MaxPageSize),
			PageToken: pageToken,
		})
		if err != nil {
			return nil, err
		}
		objects = append(objects, listRes.Objects...)
		if listRes.NextPageToken == "" {
			return objects, nil
		}
		if listRes.NextPageToken == pageToken {
			return nil, fmt.Errorf("pagination stalled: repeated page token %q", pageToken)
		}
		pageToken = listRes.NextPageToken
	}
}

func (s *Service) quarantineOrphanFolder(ctx context.Context, folder string) {
	timestamp := quarantineTimestampFunc()
	quarantineName := fmt.Sprintf("%s%s-%d", quarantineFolderPrefix, folder, timestamp)
	s.Logger.WarnContext(ctx, "object_reconcile", "quarantining orphan folder (no DB row)", logging.String("folder", folder), logging.String("quarantine_name", quarantineName))
	if err := s.objectStorage.RenameObjectFolder(folder, quarantineName); err != nil {
		s.Logger.WarnContext(ctx, "object_reconcile", "quarantine rename failed, deleting orphan folder", logging.String("folder", folder), logging.ErrorField(err))
		if deleteErr := s.objectStorage.DeleteObjectFolder(folder); deleteErr != nil {
			s.Logger.ErrorContext(ctx, "object_reconcile", "failed to delete orphan folder", logging.String("folder", folder), logging.ErrorField(deleteErr))
		}
	}
}

func (s *Service) repairObjectManifestFileIfNeeded(objectID string) error {
	data, err := s.objectStorage.ReadManifestFile(objectID)
	if err == nil {
		var manifest model.ObjectManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			_, repairErr := s.repairObjectManifestFile(objectID)
			return repairErr
		}
		return nil
	}
	if errors.Is(err, model.ErrNotFound) {
		_, repairErr := s.repairObjectManifestFile(objectID)
		return repairErr
	}
	return err
}

func (s *Service) ensureObjectManifestFileBestEffort(ctx context.Context, objectID, operation string) {
	if err := s.repairObjectManifestFileIfNeeded(objectID); err != nil {
		s.Logger.WarnContext(ctx, "object", "manifest file ensure failed", logging.String("object_id", objectID), logging.String("operation", operation), logging.ErrorField(err))
	}
}

// bestEffortSyncManifest rebuilds manifest.json from folder contents after a file mutation.
func (s *Service) bestEffortSyncManifest(ctx context.Context, objectID, caller string) (*model.ObjectManifest, error) {
	manifest, err := s.rebuildObjectManifestFile(objectID)
	if err != nil {
		s.Logger.WarnContext(ctx, "object", "manifest rebuild after mutation failed (reconcile will repair)",
			logging.String("object_id", objectID),
			logging.String("caller", caller),
			logging.ErrorField(err),
		)
		return nil, err
	}
	return manifest, nil
}

func (s *Service) rebuildObjectManifestFile(objectID string) (*model.ObjectManifest, error) {
	return s.repairObjectManifestFile(objectID)
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
	if _, repairErr := s.repairObjectManifestFile(objectID); repairErr != nil {
		return errors.Join(err, repairErr)
	}
	return nil
}

func (s *Service) repairObjectManifestFile(objectID string) (*model.ObjectManifest, error) {
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
	return manifest, nil
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
	return model.NewCoreError("OBJECT_CREATE_ROLLBACK_ERROR", strings.Join(failures, "; "))
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
	return model.NewCoreError("OBJECT_UPSERT_ROLLBACK_ERROR", strings.Join(failures, "; "))
}
