package service

import (
	"context"
	"errors"
	"io"

	"github.com/anomalyco/atlas-core/services/functions/internal/gateway"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/objectstreaming"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func bestEffortPublishObjectUpdated(ctx context.Context, log *logging.Logger, objectID string, err error) {
	if err == nil {
		return
	}
	if log != nil {
		log.WarnContext(ctx, "service.object_streaming", "object updated publish failed after committed mutation",
			logging.String("object_id", objectID),
			logging.ErrorField(err),
		)
	}
}

func (s *Server) WriteObjectFile(stream functionsv1.AtlasFunctionsService_WriteObjectFileServer) error {
	gateway, ok := s.funcs.Object.StreamingGateway()
	if !ok {
		return status.Error(codes.Internal, "streaming object gateway is not configured")
	}
	firstChunk, metadata, err := objectstreaming.ReceiveFirstWriteChunk(stream)
	if err != nil {
		return err
	}
	upload, err := gateway.OpenWriteFileStream(stream.Context(), metadata.ObjectID, metadata.Filename, metadata.ExpectedSize)
	if err != nil {
		return s.status(stream.Context(), err)
	}
	result, err := processForwardWriteChunks(stream, upload, metadata, firstChunk.GetData(), firstChunk.GetFinalChunk(), objectstreaming.MaxChunkPayloadBytes)
	if err != nil {
		if closeErr := upload.CloseSend(); closeErr != nil {
			s.log.WarnContext(stream.Context(), "service.object_streaming", "upload CloseSend failed after write error",
				logging.String("object_id", metadata.ObjectID),
				logging.String("filename", metadata.Filename),
				logging.ErrorField(closeErr),
			)
		}
		return s.status(stream.Context(), err)
	}
	bestEffortPublishObjectUpdated(stream.Context(), s.log, metadata.ObjectID, s.publishObjectUpdated(stream.Context(), metadata.ObjectID))
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
	firstChunk, metadata, err := objectstreaming.ReceiveFirstAppendChunk(stream)
	if err != nil {
		return err
	}
	upload, err := gateway.OpenAppendFileStream(
		stream.Context(),
		metadata.ObjectID,
		metadata.Filename,
		metadata.CurrentExpectedSize,
		metadata.ExpectedSize,
	)
	if err != nil {
		return s.status(stream.Context(), err)
	}
	result, err := processForwardAppendChunks(stream, upload, metadata, firstChunk.GetData(), firstChunk.GetFinalChunk(), objectstreaming.MaxChunkPayloadBytes)
	if err != nil {
		if closeErr := upload.CloseSend(); closeErr != nil {
			s.log.WarnContext(stream.Context(), "service.object_streaming", "upload CloseSend failed after append error",
				logging.String("object_id", metadata.ObjectID),
				logging.String("filename", metadata.Filename),
				logging.ErrorField(closeErr),
			)
		}
		return s.status(stream.Context(), err)
	}
	bestEffortPublishObjectUpdated(stream.Context(), s.log, metadata.ObjectID, s.publishObjectUpdated(stream.Context(), metadata.ObjectID))
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

func processForwardWriteChunks(
	stream interface {
		Recv() (*sharedv1.WriteFileChunk, error)
	},
	upload gateway.ObjectFileUploadStream,
	file objectstreaming.WriteFileMetadata,
	firstData []byte,
	firstFinalChunk bool,
	maxBytes int64,
) (gateway.ManifestResult, error) {
	var result gateway.ManifestResult
	err := objectstreaming.ProcessWriteChunks(
		stream.Recv,
		file,
		firstData,
		firstFinalChunk,
		maxBytes,
		objectstreaming.NewForwardWriteSink(file.ExpectedSize, upload.SendChunk, func() error {
			var err error
			result, err = upload.CloseAndRecv()
			return err
		}),
	)
	if err != nil {
		return gateway.ManifestResult{}, err
	}
	return result, nil
}

func processForwardAppendChunks(
	stream interface {
		Recv() (*sharedv1.AppendFileChunk, error)
	},
	upload gateway.ObjectFileUploadStream,
	file objectstreaming.AppendFileMetadata,
	firstData []byte,
	firstFinalChunk bool,
	maxBytes int64,
) (gateway.ManifestResult, error) {
	var result gateway.ManifestResult
	err := objectstreaming.ProcessAppendChunks(
		stream.Recv,
		file,
		firstData,
		firstFinalChunk,
		maxBytes,
		objectstreaming.NewForwardAppendSink(file, upload.SendChunk, func() error {
			var err error
			result, err = upload.CloseAndRecv()
			return err
		}),
	)
	if err != nil {
		return gateway.ManifestResult{}, err
	}
	return result, nil
}

func proxyReadChunks(download gateway.ObjectFileDownloadStream, send func(*sharedv1.FileChunk) error) error {
	finalSeen := false
	for {
		data, finalChunk, totalSize, err := download.RecvChunk()
		if errors.Is(err, io.EOF) {
			if !finalSeen {
				return io.ErrUnexpectedEOF
			}
			return nil
		}
		if err != nil {
			return err
		}
		if err := send(&sharedv1.FileChunk{Data: data, FinalChunk: finalChunk, TotalSize: totalSize}); err != nil {
			return err
		}
		if finalChunk {
			finalSeen = true
			return nil
		}
	}
}
