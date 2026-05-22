package objectstreaming

import (
	"context"
	"errors"
	"fmt"
	"io"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MaxChunkPayloadBytes is the maximum allowed per-chunk payload size for
// WriteObjectFile and AppendObjectFile RPCs. The value is below gRPC's default
// 4 MiB message-size ceiling to leave room for protobuf framing overhead.
const MaxChunkPayloadBytes = 4*1024*1024 - 4096 // 4 MiB − 4 KiB

// DefaultChunkSize is the read/write chunk size used when callers omit chunk_size.
const DefaultChunkSize = 64 * 1024

// WriteFileMetadata is parsed from the first WriteObjectFile chunk.
type WriteFileMetadata struct {
	ObjectID     string
	Filename     string
	ExpectedSize int64
}

// AppendFileMetadata is parsed from the first AppendObjectFile chunk.
type AppendFileMetadata struct {
	WriteFileMetadata
	CurrentExpectedSize int64
}

func ReceiveFirstWriteChunk(stream interface {
	Context() context.Context
	Recv() (*sharedv1.WriteFileChunk, error)
}) (*sharedv1.WriteFileChunk, WriteFileMetadata, error) {
	chunk, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return nil, WriteFileMetadata{}, status.Error(codes.InvalidArgument, "at least one chunk is required")
	}
	if err != nil {
		return nil, WriteFileMetadata{}, err
	}
	file := WriteFileMetadata{
		ObjectID:     chunk.GetObjectId(),
		Filename:     chunk.GetFilename(),
		ExpectedSize: chunk.GetExpectedSize(),
	}
	if file.ObjectID == "" {
		return nil, WriteFileMetadata{}, status.Error(codes.InvalidArgument, "object_id is required")
	}
	if file.Filename == "" {
		return nil, WriteFileMetadata{}, status.Error(codes.InvalidArgument, "filename is required")
	}
	return chunk, file, nil
}

func ReceiveFirstAppendChunk(stream interface {
	Context() context.Context
	Recv() (*sharedv1.AppendFileChunk, error)
}) (*sharedv1.AppendFileChunk, AppendFileMetadata, error) {
	chunk, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return nil, AppendFileMetadata{}, status.Error(codes.InvalidArgument, "at least one chunk is required")
	}
	if err != nil {
		return nil, AppendFileMetadata{}, err
	}
	file := AppendFileMetadata{
		WriteFileMetadata: WriteFileMetadata{
			ObjectID:     chunk.GetObjectId(),
			Filename:     chunk.GetFilename(),
			ExpectedSize: chunk.GetExpectedSize(),
		},
		CurrentExpectedSize: chunk.GetCurrentExpectedSize(),
	}
	if file.ObjectID == "" {
		return nil, AppendFileMetadata{}, status.Error(codes.InvalidArgument, "object_id is required")
	}
	if file.Filename == "" {
		return nil, AppendFileMetadata{}, status.Error(codes.InvalidArgument, "filename is required")
	}
	return chunk, file, nil
}

func ValidateWriteChunkMetadata(file WriteFileMetadata, objectID, filename string, expectedSize int64) error {
	if objectID != file.ObjectID {
		return status.Error(codes.InvalidArgument, "object_id must match across all chunks")
	}
	if filename != file.Filename {
		return status.Error(codes.InvalidArgument, "filename must match across all chunks")
	}
	if expectedSize != file.ExpectedSize {
		return status.Error(codes.InvalidArgument, "expected_size must match across all chunks")
	}
	return nil
}

func ValidateChunkSize(data []byte, maxBytes int64) error {
	if int64(len(data)) > maxBytes {
		return status.Error(codes.ResourceExhausted, fmt.Sprintf("chunk size %d exceeds maximum of %d bytes", len(data), maxBytes))
	}
	return nil
}
