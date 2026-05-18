package gateway

import (
	"context"

	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

// ObjectGateway is the functions-service port for object metadata, files, and reconcile.
type ObjectGateway interface {
	store.ObjectStore
	EnsureObjectCreated(ctx context.Context, object *model.Object) error
	WriteFile(ctx context.Context, objectID, filename string, data []byte) (ManifestResult, error)
	AppendFile(ctx context.Context, objectID, filename string, data []byte) (ManifestResult, error)
	ReadFile(ctx context.Context, objectID, filename string) ([]byte, error)
	DeleteFile(ctx context.Context, objectID, filename string) (ManifestResult, error)
	ListFiles(ctx context.Context, objectID string) ([]string, error)
	Reconcile(ctx context.Context) error
}

// ManifestResult carries manifest state returned from file operations.
type ManifestResult struct {
	Manifest          *model.ObjectManifest
	ManifestCurrent   bool
	ManifestSyncError string
}

// ObjectFileUploadStream streams object file writes to the gateway implementation.
type ObjectFileUploadStream interface {
	SendChunk(data []byte, finalChunk bool) error
	CloseAndRecv() (ManifestResult, error)
	CloseSend() error
}

// ObjectFileDownloadStream streams object file reads from the gateway implementation.
type ObjectFileDownloadStream interface {
	RecvChunk() (data []byte, finalChunk bool, totalSize int64, err error)
}

// StreamingObjectGateway supports chunked object file RPCs.
type StreamingObjectGateway interface {
	OpenWriteFileStream(ctx context.Context, objectID, filename string, expectedSize int64) (ObjectFileUploadStream, error)
	OpenAppendFileStream(ctx context.Context, objectID, filename string, currentExpectedSize, expectedSize int64) (ObjectFileUploadStream, error)
	OpenReadFileStream(ctx context.Context, objectID, filename string, chunkSize int64) (ObjectFileDownloadStream, error)
}
