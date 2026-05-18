package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/anomalyco/atlas-core/services/functions/internal/gateway"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const MAX_OBJECT_FILE_CHUNK_BYTES = 4*1024*1024 - 4096 // 4 MiB − 4 KiB

func (s *Server) WriteObjectFile(stream functionsv1.AtlasFunctionsService_WriteObjectFileServer) error {
	gateway, ok := s.funcs.Object.StreamingGateway()
	if !ok {
		return status.Error(codes.Internal, "streaming object gateway is not configured")
	}
	firstChunk, metadata, err := receiveFirstWriteChunk(stream)
	if err != nil {
		return err
	}
	upload, err := gateway.OpenWriteFileStream(stream.Context(), metadata.objectID, metadata.filename, metadata.expectedSize)
	if err != nil {
		return s.status(stream.Context(), err)
	}
	result, err := forwardWriteChunks(stream, upload, metadata, firstChunk.GetData(), firstChunk.GetFinalChunk(), MAX_OBJECT_FILE_CHUNK_BYTES)
	if err != nil {
		if closeErr := upload.CloseSend(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := s.funcs.Object.PublishObjectUpdated(stream.Context(), metadata.objectID); err != nil {
		return s.status(stream.Context(), err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{
		Manifest:          pbconv.ManifestToProto(result.Manifest),
		ManifestCurrent:   result.ManifestCurrent,
		ManifestSyncError: result.ManifestSyncError,
	})
}
func (s *Server) AppendObjectFile(stream functionsv1.AtlasFunctionsService_AppendObjectFileServer) error {
	gateway, ok := s.funcs.Object.StreamingGateway()
	if !ok {
		return status.Error(codes.Internal, "streaming object gateway is not configured")
	}
	firstChunk, metadata, err := receiveFirstAppendChunk(stream)
	if err != nil {
		return err
	}
	upload, err := gateway.OpenAppendFileStream(
		stream.Context(),
		metadata.objectID,
		metadata.filename,
		metadata.currentExpectedSize,
		metadata.expectedSize,
	)
	if err != nil {
		return s.status(stream.Context(), err)
	}
	result, err := forwardAppendChunks(stream, upload, metadata, firstChunk.GetData(), firstChunk.GetFinalChunk(), MAX_OBJECT_FILE_CHUNK_BYTES)
	if err != nil {
		if closeErr := upload.CloseSend(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := s.funcs.Object.PublishObjectUpdated(stream.Context(), metadata.objectID); err != nil {
		return s.status(stream.Context(), err)
	}
	return stream.SendAndClose(&sharedv1.ObjectManifestResponse{
		Manifest:          pbconv.ManifestToProto(result.Manifest),
		ManifestCurrent:   result.ManifestCurrent,
		ManifestSyncError: result.ManifestSyncError,
	})
}
func (s *Server) ReadObjectFile(req *sharedv1.ReadFileRequest, stream functionsv1.AtlasFunctionsService_ReadObjectFileServer) error {
	gateway, ok := s.funcs.Object.StreamingGateway()
	if !ok {
		return status.Error(codes.Internal, "streaming object gateway is not configured")
	}
	download, err := gateway.OpenReadFileStream(stream.Context(), req.GetObjectId(), req.GetFilename(), req.GetChunkSize())
	if err != nil {
		return s.status(stream.Context(), err)
	}
	return proxyReadChunks(download, stream.Send)
}

type receivedWriteFile struct {
	objectID     string
	filename     string
	expectedSize int64
}

type receivedAppendFile struct {
	receivedWriteFile
	currentExpectedSize int64
}

func receiveFirstWriteChunk(stream interface {
	Context() context.Context
	Recv() (*sharedv1.WriteFileChunk, error)
}) (*sharedv1.WriteFileChunk, receivedWriteFile, error) {
	chunk, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return nil, receivedWriteFile{}, status.Error(codes.InvalidArgument, "at least one chunk is required")
	}
	if err != nil {
		return nil, receivedWriteFile{}, err
	}
	file := receivedWriteFile{
		objectID:     chunk.GetObjectId(),
		filename:     chunk.GetFilename(),
		expectedSize: chunk.GetExpectedSize(),
	}
	if err := validateWriteChunkMetadata(file, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
		return nil, receivedWriteFile{}, err
	}
	if file.objectID == "" {
		return nil, receivedWriteFile{}, status.Error(codes.InvalidArgument, "object_id is required")
	}
	if file.filename == "" {
		return nil, receivedWriteFile{}, status.Error(codes.InvalidArgument, "filename is required")
	}
	return chunk, file, nil
}

func receiveFirstAppendChunk(
	stream interface {
		Context() context.Context
		Recv() (*sharedv1.AppendFileChunk, error)
	},
) (*sharedv1.AppendFileChunk, receivedAppendFile, error) {
	chunk, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return nil, receivedAppendFile{}, status.Error(codes.InvalidArgument, "at least one chunk is required")
	}
	if err != nil {
		return nil, receivedAppendFile{}, err
	}
	file := receivedAppendFile{
		receivedWriteFile: receivedWriteFile{
			objectID:     chunk.GetObjectId(),
			filename:     chunk.GetFilename(),
			expectedSize: chunk.GetExpectedSize(),
		},
		currentExpectedSize: chunk.GetCurrentExpectedSize(),
	}
	if err := validateWriteChunkMetadata(file.receivedWriteFile, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
		return nil, receivedAppendFile{}, err
	}
	if file.objectID == "" {
		return nil, receivedAppendFile{}, status.Error(codes.InvalidArgument, "object_id is required")
	}
	if file.filename == "" {
		return nil, receivedAppendFile{}, status.Error(codes.InvalidArgument, "filename is required")
	}
	return chunk, file, nil
}

func validateWriteChunkMetadata(file receivedWriteFile, objectID, filename string, expectedSize int64) error {
	if objectID != file.objectID {
		return status.Error(codes.InvalidArgument, "object_id must match across all chunks")
	}
	if filename != file.filename {
		return status.Error(codes.InvalidArgument, "filename must match across all chunks")
	}
	if expectedSize != 0 && expectedSize != file.expectedSize {
		return status.Error(codes.InvalidArgument, "expected_size must match across all chunks")
	}
	return nil
}

func forwardWriteChunks(
	stream interface {
		Recv() (*sharedv1.WriteFileChunk, error)
	},
	upload gateway.ObjectFileUploadStream,
	file receivedWriteFile,
	firstData []byte,
	firstFinalChunk bool,
	maxBytes int64,
) (gateway.ManifestResult, error) {
	totalBytes := int64(0)
	next := func(data []byte, finalChunk bool) (gateway.ManifestResult, error) {
		if err := validateChunkSize(data, maxBytes); err != nil {
			return gateway.ManifestResult{}, err
		}
		totalBytes += int64(len(data))
		if err := upload.SendChunk(data, finalChunk); err != nil {
			return gateway.ManifestResult{}, err
		}
		if !finalChunk {
			return gateway.ManifestResult{ManifestCurrent: true}, nil
		}
		if file.expectedSize != 0 && totalBytes != file.expectedSize {
			return gateway.ManifestResult{}, status.Error(codes.InvalidArgument, fmt.Sprintf("expected_size mismatch: got %d bytes, expected %d", totalBytes, file.expectedSize))
		}
		if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
			if err != nil {
				return gateway.ManifestResult{}, err
			}
			return gateway.ManifestResult{}, status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		return upload.CloseAndRecv()
	}
	if result, err := next(firstData, firstFinalChunk); result.Manifest != nil || result.ManifestSyncError != "" || err != nil {
		return result, err
	}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return gateway.ManifestResult{}, status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
		}
		if err != nil {
			return gateway.ManifestResult{}, err
		}
		if err := validateWriteChunkMetadata(file, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
			return gateway.ManifestResult{}, err
		}
		if result, err := next(chunk.GetData(), chunk.GetFinalChunk()); result.Manifest != nil || result.ManifestSyncError != "" || err != nil {
			return result, err
		}
	}
}

func forwardAppendChunks(
	stream interface {
		Recv() (*sharedv1.AppendFileChunk, error)
	},
	upload gateway.ObjectFileUploadStream,
	file receivedAppendFile,
	firstData []byte,
	firstFinalChunk bool,
	maxBytes int64,
) (gateway.ManifestResult, error) {
	totalBytes := int64(0)
	next := func(data []byte, finalChunk bool) (gateway.ManifestResult, error) {
		if err := validateChunkSize(data, maxBytes); err != nil {
			return gateway.ManifestResult{}, err
		}
		totalBytes += int64(len(data))
		if err := upload.SendChunk(data, finalChunk); err != nil {
			return gateway.ManifestResult{}, err
		}
		if !finalChunk {
			return gateway.ManifestResult{ManifestCurrent: true}, nil
		}
		if file.expectedSize != 0 && file.currentExpectedSize+totalBytes != file.expectedSize {
			return gateway.ManifestResult{}, status.Error(codes.InvalidArgument, fmt.Sprintf(
				"expected_size mismatch: got %d bytes after append, expected %d",
				file.currentExpectedSize+totalBytes, file.expectedSize))
		}
		if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
			if err != nil {
				return gateway.ManifestResult{}, err
			}
			return gateway.ManifestResult{}, status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		return upload.CloseAndRecv()
	}
	if result, err := next(firstData, firstFinalChunk); result.Manifest != nil || result.ManifestSyncError != "" || err != nil {
		return result, err
	}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return gateway.ManifestResult{}, status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
		}
		if err != nil {
			return gateway.ManifestResult{}, err
		}
		if err := validateWriteChunkMetadata(file.receivedWriteFile, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
			return gateway.ManifestResult{}, err
		}
		if chunk.GetCurrentExpectedSize() != file.currentExpectedSize {
			return gateway.ManifestResult{}, status.Error(codes.InvalidArgument, "current_expected_size must match across all chunks")
		}
		if result, err := next(chunk.GetData(), chunk.GetFinalChunk()); result.Manifest != nil || result.ManifestSyncError != "" || err != nil {
			return result, err
		}
	}
}

func validateChunkSize(data []byte, maxBytes int64) error {
	if int64(len(data)) > maxBytes {
		return status.Error(codes.ResourceExhausted, fmt.Sprintf("chunk size %d exceeds maximum of %d bytes", len(data), maxBytes))
	}
	return nil
}

func proxyReadChunks(download gateway.ObjectFileDownloadStream, send func(*sharedv1.FileChunk) error) error {
	for {
		data, finalChunk, totalSize, err := download.RecvChunk()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := send(&sharedv1.FileChunk{Data: data, FinalChunk: finalChunk, TotalSize: totalSize}); err != nil {
			return err
		}
		if finalChunk {
			return nil
		}
	}
}
