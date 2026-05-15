package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type ObjectGateway interface {
	store.ObjectStore
	EnsureObjectCreated(ctx context.Context, object *model.Object) error
	WriteFile(ctx context.Context, objectID, filename string, data []byte) (MutationResult, error)
	AppendFile(ctx context.Context, objectID, filename string, data []byte) (MutationResult, error)
	ReadFile(ctx context.Context, objectID, filename string) ([]byte, error)
	DeleteFile(ctx context.Context, objectID, filename string) (MutationResult, error)
	ListFiles(ctx context.Context, objectID string) ([]string, error)
	Reconcile(ctx context.Context) error
}

type MutationResult struct {
	Manifest        *model.ObjectManifest
	ManifestCurrent bool
	SyncError       string
}

type localObjectGateway struct {
	metadata store.ObjectStore
	files    store.ObjectStorageStore
}

func (g *localObjectGateway) CreateObject(ctx context.Context, object *model.Object) error {
	if err := g.metadata.CreateObject(ctx, object); err != nil {
		return err
	}
	if err := g.ensureObjectFolderReady(object.ObjectID); err != nil {
		if rollbackErr := rollbackObjectCreate(ctx, g.metadata, g.files, object.ObjectID); rollbackErr != nil {
			return errors.Join(model.NewCoreError("OBJECT_CREATE_ERROR", "failed to initialize object storage"), err, rollbackErr)
		}
		return err
	}
	return g.syncObjectManifestFromFilesystemIgnoringErrors(ctx, object.ObjectID)
}

func (g *localObjectGateway) EnsureObjectCreated(ctx context.Context, object *model.Object) error {
	metadataCreated := false
	if _, err := g.metadata.GetObject(ctx, object.ObjectID); err != nil {
		if !errors.Is(err, model.ErrNotFound) {
			return err
		}
		if err := g.metadata.CreateObject(ctx, object); err != nil {
			if !errors.Is(err, model.ErrConflict) {
				return err
			}
		} else {
			metadataCreated = true
		}
	}
	if err := g.ensureObjectFolderReady(object.ObjectID); err != nil {
		if metadataCreated {
			if rollbackErr := rollbackObjectCreate(ctx, g.metadata, g.files, object.ObjectID); rollbackErr != nil {
				return errors.Join(model.NewCoreError("OBJECT_CREATE_ERROR", "failed to recover object storage"), err, rollbackErr)
			}
		}
		return err
	}
	return g.syncObjectManifestFromFilesystemIgnoringErrors(ctx, object.ObjectID)
}

func (g *localObjectGateway) GetObject(ctx context.Context, objectID string) (*model.Object, error) {
	return g.metadata.GetObject(ctx, objectID)
}

func (g *localObjectGateway) ListObjects(ctx context.Context, filters ...store.ObjectFilter) ([]model.Object, error) {
	return g.metadata.ListObjects(ctx, filters...)
}

func (g *localObjectGateway) UpdateObject(ctx context.Context, object *model.Object) error {
	return g.metadata.UpdateObject(ctx, object)
}

func (g *localObjectGateway) DeleteObject(ctx context.Context, objectID string) error {
	object, err := g.metadata.GetObject(ctx, objectID)
	if err != nil {
		return err
	}
	if err := g.metadata.DeleteObject(ctx, objectID); err != nil {
		return err
	}
	if err := g.files.DeleteObjectFolder(objectID); err != nil {
		if restoreErr := g.metadata.UpsertObject(ctx, object); restoreErr != nil {
			return errors.Join(model.NewCoreError("OBJECT_DELETE_ERROR", "failed to delete object storage and restore metadata"), err, restoreErr)
		}
		return err
	}
	return nil
}

func (g *localObjectGateway) UpsertObject(ctx context.Context, object *model.Object) error {
	_, existingErr := g.metadata.GetObject(ctx, object.ObjectID)
	objectExists := existingErr == nil
	if existingErr != nil && !errors.Is(existingErr, model.ErrNotFound) {
		return existingErr
	}
	if err := g.metadata.UpsertObject(ctx, object); err != nil {
		return err
	}
	folderExists, err := g.files.ObjectFolderExists(object.ObjectID)
	if err != nil {
		if !objectExists {
			if rollbackErr := g.metadata.DeleteObject(ctx, object.ObjectID); rollbackErr != nil {
				return errors.Join(model.NewCoreError("OBJECT_UPSERT_ERROR", "failed to inspect object storage and rollback metadata"), err, rollbackErr)
			}
		}
		return err
	}
	if !folderExists {
		if err := g.files.CreateObjectFolder(object.ObjectID); err != nil {
			if rollbackErr := rollbackObjectUpsert(ctx, g.metadata, g.files, object.ObjectID, !objectExists); rollbackErr != nil {
				return errors.Join(model.NewCoreError("OBJECT_UPSERT_ERROR", "failed to initialize object storage"), err, rollbackErr)
			}
			return err
		}
	}
	return g.syncObjectManifestFromFilesystemIgnoringErrors(ctx, object.ObjectID)
}

func (g *localObjectGateway) UpdateObjectManifest(ctx context.Context, objectID string, manifest *model.ObjectManifest, updatedAt ...time.Time) error {
	if _, err := g.metadata.GetObject(ctx, objectID); err != nil {
		return err
	}
	manifest = model.NormalizeManifest(manifest)
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return model.NewCoreError("MANIFEST_ERROR", "failed to marshal manifest: "+err.Error())
	}
	if err := g.files.WriteManifestFile(objectID, manifestBytes); err != nil {
		return err
	}
	now := time.Now().UTC()
	if len(updatedAt) > 0 {
		now = updatedAt[0].UTC()
	}
	if err := g.metadata.UpdateObjectManifest(ctx, objectID, manifest, now); err != nil {
		return model.NewCoreError("MANIFEST_CACHE_SYNC_ERROR", "manifest written to filesystem but failed to update database cache: "+err.Error())
	}
	return nil
}

func (g *localObjectGateway) GetObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	if _, err := g.metadata.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	data, err := g.files.ReadManifestFile(objectID)
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

func (g *localObjectGateway) WriteFile(ctx context.Context, objectID, filename string, data []byte) (MutationResult, error) {
	if err := g.files.ValidateSafeObjectPath(objectID, filename); err != nil {
		return MutationResult{}, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := g.metadata.GetObject(ctx, objectID); err != nil {
		return MutationResult{}, err
	}
	if err := g.files.WriteObjectFile(objectID, filename, data); err != nil {
		return MutationResult{}, err
	}
	return g.syncManifestWithState(ctx, objectID), nil
}

func (g *localObjectGateway) AppendFile(ctx context.Context, objectID, filename string, data []byte) (MutationResult, error) {
	if err := g.files.ValidateSafeObjectPath(objectID, filename); err != nil {
		return MutationResult{}, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := g.metadata.GetObject(ctx, objectID); err != nil {
		return MutationResult{}, err
	}
	if err := g.files.AppendObjectFile(objectID, filename, data); err != nil {
		return MutationResult{}, err
	}
	return g.syncManifestWithState(ctx, objectID), nil
}

func (g *localObjectGateway) ReadFile(ctx context.Context, objectID, filename string) ([]byte, error) {
	if err := g.files.ValidateSafeObjectPath(objectID, filename); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := g.metadata.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	return g.files.ReadObjectFile(objectID, filename)
}

func (g *localObjectGateway) DeleteFile(ctx context.Context, objectID, filename string) (MutationResult, error) {
	if err := g.files.ValidateSafeObjectPath(objectID, filename); err != nil {
		return MutationResult{}, model.NewFieldError("INVALID_INPUT", err.Error(), "path")
	}
	if _, err := g.metadata.GetObject(ctx, objectID); err != nil {
		return MutationResult{}, err
	}
	if err := g.files.DeleteObjectFile(objectID, filename); err != nil {
		return MutationResult{}, err
	}
	return g.syncManifestWithState(ctx, objectID), nil
}

func (g *localObjectGateway) ListFiles(ctx context.Context, objectID string) ([]string, error) {
	if err := validateObjectID(objectID); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	if _, err := g.metadata.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	return g.files.ListObjectFolderFiles(objectID)
}

func (g *localObjectGateway) Reconcile(ctx context.Context) error {
	objects, err := g.metadata.ListObjects(ctx)
	if err != nil {
		return fmt.Errorf("list database objects: %w", err)
	}
	folders, err := g.files.ListObjectFolders()
	if err != nil {
		return fmt.Errorf("list object folders: %w", err)
	}
	indexed := map[string]model.Object{}
	for _, object := range objects {
		indexed[object.ObjectID] = object
	}
	for _, folder := range folders {
		if err := validateObjectID(folder); err != nil {
			if deleteErr := g.files.DeleteObjectFolder(folder); deleteErr != nil {
				return fmt.Errorf("delete invalid object folder %s: %w", folder, deleteErr)
			}
			continue
		}
		if _, ok := indexed[folder]; !ok {
			if err := g.restoreOrphanObjectFromFilesystem(ctx, folder); err != nil {
				if !errors.Is(err, model.ErrNotFound) {
					return fmt.Errorf("restore orphan object folder %s: %w", folder, err)
				}
				if _, dbErr := g.metadata.GetObject(ctx, folder); dbErr == nil {
					continue
				} else if !errors.Is(dbErr, model.ErrNotFound) {
					return fmt.Errorf("re-check object existence for %s: %w", folder, dbErr)
				}
				if err := g.files.DeleteObjectFolder(folder); err != nil {
					return fmt.Errorf("delete orphan object folder %s: %w", folder, err)
				}
			}
			continue
		}
		if err := g.syncObjectManifestFromFilesystemWithRepair(ctx, folder); err != nil {
			return fmt.Errorf("sync object manifest %s: %w", folder, err)
		}
		delete(indexed, folder)
	}
	remaining := make([]string, 0, len(indexed))
	for objectID := range indexed {
		remaining = append(remaining, objectID)
	}
	sort.Strings(remaining)
	for _, objectID := range remaining {
		if err := g.files.CreateObjectFolder(objectID); err != nil {
			return fmt.Errorf("create missing object folder %s: %w", objectID, err)
		}
		if err := g.syncObjectManifestFromFilesystemWithRepair(ctx, objectID); err != nil {
			return fmt.Errorf("sync recreated object manifest %s: %w", objectID, err)
		}
	}
	return nil
}

func (g *localObjectGateway) syncManifestWithState(ctx context.Context, objectID string) MutationResult {
	manifest, err := g.rebuildAndSyncObjectManifest(ctx, objectID)
	if err != nil {
		return MutationResult{ManifestCurrent: false, SyncError: err.Error()}
	}
	return MutationResult{Manifest: manifest, ManifestCurrent: true}
}

func (g *localObjectGateway) syncObjectManifestFromFilesystemIgnoringErrors(ctx context.Context, objectID string) error {
	_ = g.syncObjectManifestFromFilesystem(ctx, objectID)
	return nil
}

func (g *localObjectGateway) syncObjectManifestFromFilesystem(ctx context.Context, objectID string) error {
	data, err := g.files.ReadManifestFile(objectID)
	if err != nil {
		return err
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("%w for %s: %w", errDecodeObjectManifest, objectID, err)
	}
	manifestPtr := model.NormalizeManifest(&manifest)
	cachedManifest, err := g.metadata.GetObjectManifest(ctx, objectID)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return err
	}
	if cachedManifest != nil && cachedManifest.Version == manifestPtr.Version {
		return nil
	}
	return g.metadata.UpdateObjectManifest(ctx, objectID, manifestPtr, time.Now().UTC())
}

func (g *localObjectGateway) syncObjectManifestFromFilesystemWithRepair(ctx context.Context, objectID string) error {
	err := g.syncObjectManifestFromFilesystem(ctx, objectID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, model.ErrNotFound) && !errors.Is(err, errDecodeObjectManifest) {
		return err
	}
	if repairErr := g.repairObjectManifestFile(objectID); repairErr != nil {
		return errors.Join(err, repairErr)
	}
	return g.syncObjectManifestFromFilesystem(ctx, objectID)
}

func (g *localObjectGateway) restoreOrphanObjectFromFilesystem(ctx context.Context, objectID string) error {
	if err := validateObjectID(objectID); err != nil {
		return err
	}
	existingObject, err := g.metadata.GetObject(ctx, objectID)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("check existing object metadata: %w", err)
	}
	manifest, err := g.readObjectManifestFromFilesystem(objectID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			if existingObject == nil {
				return err
			}
		} else if !errors.Is(err, errDecodeObjectManifest) {
			return err
		}
		if err := g.repairObjectManifestFile(objectID); err != nil {
			return err
		}
		manifest, err = g.readObjectManifestFromFilesystem(objectID)
		if err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	if existingObject != nil {
		return g.metadata.UpdateObjectManifest(ctx, objectID, manifest, now)
	}
	restored := &model.Object{ObjectID: objectID, Type: model.ObjectTypeLog, OwnerType: model.OwnerTypeSystem, OwnerID: "system", JSON: []byte("{}"), CreatedAt: now, UpdatedAt: now}
	if err := g.metadata.CreateObject(ctx, restored); err != nil && !errors.Is(err, model.ErrConflict) {
		return fmt.Errorf("create restored object metadata: %w", err)
	}
	return g.metadata.UpdateObjectManifest(ctx, objectID, manifest, now)
}

func (g *localObjectGateway) ensureObjectFolderReady(objectID string) error {
	err := g.files.CreateObjectFolder(objectID)
	if err == nil {
		return nil
	}
	exists, existsErr := g.files.ObjectFolderExists(objectID)
	if existsErr != nil {
		return errors.Join(err, existsErr)
	}
	if !exists {
		return err
	}
	if repairErr := g.repairObjectManifestFile(objectID); repairErr != nil {
		return errors.Join(err, repairErr)
	}
	return nil
}

func (g *localObjectGateway) repairObjectManifestFile(objectID string) error {
	manifest, err := g.rebuildObjectManifestFromFilesystem(objectID)
	if err != nil {
		return err
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal rebuilt manifest for %s: %w", objectID, err)
	}
	if err := g.files.WriteManifestFile(objectID, manifestData); err != nil {
		return fmt.Errorf("rewrite manifest for %s: %w", objectID, err)
	}
	return nil
}

func (g *localObjectGateway) readObjectManifestFromFilesystem(objectID string) (*model.ObjectManifest, error) {
	data, err := g.files.ReadManifestFile(objectID)
	if err != nil {
		return nil, err
	}
	var manifest model.ObjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%w for %s: %w", errDecodeObjectManifest, objectID, err)
	}
	return model.NormalizeManifest(&manifest), nil
}

func (g *localObjectGateway) rebuildObjectManifestFromFilesystem(objectID string) (*model.ObjectManifest, error) {
	files, err := g.files.ListObjectFolderFiles(objectID)
	if err != nil {
		return nil, fmt.Errorf("list object files for %s: %w", objectID, err)
	}
	manifest := &model.ObjectManifest{Files: make(map[string]model.ObjectFileInfo, len(files))}
	for _, name := range files {
		info, err := g.files.GetObjectFileInfo(objectID, name)
		if err != nil {
			return nil, fmt.Errorf("stat object file %s/%s: %w", objectID, name, err)
		}
		manifest.Files[name] = info
	}
	return model.NormalizeManifest(manifest), nil
}

func (g *localObjectGateway) rebuildAndSyncObjectManifest(ctx context.Context, objectID string) (*model.ObjectManifest, error) {
	manifest, err := g.rebuildObjectManifestFromFilesystem(objectID)
	if err != nil {
		return nil, err
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal rebuilt manifest for %s: %w", objectID, err)
	}
	if err := g.files.WriteManifestFile(objectID, manifestData); err != nil {
		return nil, fmt.Errorf("rewrite manifest for %s: %w", objectID, err)
	}
	if err := g.metadata.UpdateObjectManifest(ctx, objectID, manifest, time.Now().UTC()); err != nil {
		return nil, err
	}
	return manifest, nil
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
