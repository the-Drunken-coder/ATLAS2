package datastorageclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

// NewRemoteStorageStore returns an ObjectStorageStore that delegates file
// operations to the datastorage service via gRPC instead of accessing the
// local filesystem. Folder operations are no-ops because the datastorage
// service owns the filesystem layout. Manifest operations delegate through
// the metadata store's GetObjectManifest / UpdateObjectManifest RPCs.
func NewRemoteStorageStore(
	client datastoragev1.DataStorageServiceClient,
	metadata store.ObjectStore,
) store.ObjectStorageStore {
	return &remoteStorageStore{client: client, metadata: metadata}
}

type remoteStorageStore struct {
	client   datastoragev1.DataStorageServiceClient
	metadata store.ObjectStore
}

func (r *remoteStorageStore) CreateObjectFolder(objectID string) error {
	// Folder creation is owned by the datastorage service.
	return nil
}

func (r *remoteStorageStore) ObjectFolderExists(objectID string) (bool, error) {
	// Probe by listing files — an empty result still means the object exists
	// on the datastorage side.
	_, err := r.client.ListObjectFiles(context.Background(), &sharedv1.ListObjectFilesRequest{
		ObjectId: objectID,
	})
	if err != nil {
		// If the RPC fails, assume the folder doesn't exist.
		return false, nil
	}
	return true, nil
}

func (r *remoteStorageStore) ListObjectFolders() ([]string, error) {
	// Folder enumeration has no direct gRPC equivalent; Reconcile is
	// delegated through the gateway separately, so this is only used
	// for caller-side short-circuit paths in the local gateway.
	return nil, nil
}

func (r *remoteStorageStore) DeleteObjectFolder(objectID string) error {
	// Folder cleanup is owned by the datastorage service.
	return nil
}

func (r *remoteStorageStore) WriteObjectFile(objectID, filename string, data []byte) error {
	_, err := r.client.WriteObjectFile(context.Background(), &sharedv1.WriteObjectFileRequest{
		ObjectId: objectID,
		Filename: filename,
		Data:     data,
	})
	return rpcerrors.FromStatus(err)
}

func (r *remoteStorageStore) AppendObjectFile(objectID, filename string, data []byte) error {
	_, err := r.client.AppendObjectFile(context.Background(), &sharedv1.WriteObjectFileRequest{
		ObjectId: objectID,
		Filename: filename,
		Data:     data,
	})
	return rpcerrors.FromStatus(err)
}

func (r *remoteStorageStore) ReadObjectFile(objectID, filename string) ([]byte, error) {
	resp, err := r.client.ReadObjectFile(context.Background(), &sharedv1.ReadObjectFileRequest{
		ObjectId: objectID,
		Filename: filename,
	})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return resp.GetData(), nil
}

func (r *remoteStorageStore) DeleteObjectFile(objectID, filename string) error {
	_, err := r.client.DeleteObjectFile(context.Background(), &sharedv1.ReadObjectFileRequest{
		ObjectId: objectID,
		Filename: filename,
	})
	return rpcerrors.FromStatus(err)
}

func (r *remoteStorageStore) ListObjectFolderFiles(objectID string) ([]string, error) {
	resp, err := r.client.ListObjectFiles(context.Background(), &sharedv1.ListObjectFilesRequest{
		ObjectId: objectID,
	})
	if err != nil {
		return nil, rpcerrors.FromStatus(err)
	}
	return resp.GetFilenames(), nil
}

func (r *remoteStorageStore) GetObjectFileInfo(objectID, filename string) (model.ObjectFileInfo, error) {
	// Read the file to determine size; UpdatedAt comes from the manifest.
	data, err := r.ReadObjectFile(objectID, filename)
	if err != nil {
		return model.ObjectFileInfo{}, err
	}
	return model.ObjectFileInfo{
		Size:      int64(len(data)),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (r *remoteStorageStore) ReadManifestFile(objectID string) ([]byte, error) {
	manifest, err := r.metadata.GetObjectManifest(context.Background(), objectID)
	if err != nil {
		return nil, fmt.Errorf("read manifest for %s: %w", objectID, err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest for %s: %w", objectID, err)
	}
	return data, nil
}

func (r *remoteStorageStore) WriteManifestFile(objectID string, data []byte) error {
	var manifest model.ObjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("unmarshal manifest for %s: %w", objectID, err)
	}
	return r.metadata.UpdateObjectManifest(context.Background(), objectID, model.NormalizeManifest(&manifest))
}

func (r *remoteStorageStore) ValidateSafeObjectPath(objectID, filename string) error {
	if objectID == "" {
		return fmt.Errorf("object_id is required")
	}
	if filename == "" {
		return fmt.Errorf("filename is required")
	}
	if strings.Contains(objectID, "..") || strings.Contains(filename, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	cleaned := filepath.Clean(filepath.Join(objectID, filename))
	if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return fmt.Errorf("path traversal not allowed")
	}
	return nil
}

func (r *remoteStorageStore) ReaderForObjectFile(objectID, filename string) (io.ReadCloser, error) {
	data, err := r.ReadObjectFile(objectID, filename)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
