package service

import (
	"context"

	"github.com/anomalyco/atlas-core/internal/core/model"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

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
	if err := f.rebuildAndSyncObjectManifest(ctx, objectID); err != nil {
		f.log.WarnContext(ctx, "object", "failed to sync object manifest", logging.String("object_id", objectID), logging.ErrorField(err))
	}
	return nil
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
	if err := f.rebuildAndSyncObjectManifest(ctx, objectID); err != nil {
		f.log.WarnContext(ctx, "object", "failed to sync object manifest", logging.String("object_id", objectID), logging.ErrorField(err))
	}
	return nil
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
	if err := f.rebuildAndSyncObjectManifest(ctx, objectID); err != nil {
		f.log.WarnContext(ctx, "object", "failed to sync object manifest", logging.String("object_id", objectID), logging.ErrorField(err))
	}
	return nil
}

func (f ObjectFunctions) ListFiles(ctx context.Context, objectID string) ([]string, error) {
	if err := model.ValidateObjectID(objectID); err != nil {
		return nil, model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	if _, err := f.pgStore.GetObject(ctx, objectID); err != nil {
		return nil, err
	}
	return f.objStore.ListObjectFolderFiles(objectID)
}
