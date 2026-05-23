package objectstreaming

import (
	"errors"
	"fmt"
	"io"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WriteChunkSink handles one write-stream chunk. totalBytes is the cumulative payload size
// including data. On a final chunk the sink validates totals; return finished=true when
// the sink has consumed any trailing stream messages (e.g. upload CloseAndRecv).
type WriteChunkSink func(data []byte, final bool, totalBytes int64) (finished bool, err error)

// AppendChunkSink handles one append-stream chunk. baseSize is the on-disk size before
// this RPC; totalBytes is cumulative append payload only.
type AppendChunkSink func(data []byte, final bool, totalBytes int64) (finished bool, err error)

// NewWriterSink returns a sink that writes payloads to w and validates expectedSize on the final chunk.
func NewWriterSink(w io.Writer, expectedSize int64) WriteChunkSink {
	return func(data []byte, final bool, totalBytes int64) (bool, error) {
		if len(data) > 0 {
			if _, err := w.Write(data); err != nil {
				return false, err
			}
		}
		if !final {
			return false, nil
		}
		if expectedSize != 0 && totalBytes != expectedSize {
			return false, status.Error(codes.InvalidArgument, fmt.Sprintf("expected_size mismatch: got %d bytes, expected %d", totalBytes, expectedSize))
		}
		return false, nil
	}
}

// NewAppendWriterSink returns a sink that appends payloads to w.
func NewAppendWriterSink(w io.Writer, baseSize, expectedSize int64) AppendChunkSink {
	return func(data []byte, final bool, totalBytes int64) (bool, error) {
		if len(data) > 0 {
			if _, err := w.Write(data); err != nil {
				return false, err
			}
		}
		if !final {
			return false, nil
		}
		if expectedSize != 0 && baseSize+totalBytes != expectedSize {
			return false, status.Error(codes.InvalidArgument, fmt.Sprintf(
				"expected_size mismatch: got %d bytes after append, expected %d",
				baseSize+totalBytes, expectedSize))
		}
		return false, nil
	}
}

// ProcessWriteChunks reads write chunks from recv and dispatches them to sink.
func ProcessWriteChunks(
	recv func() (*sharedv1.WriteFileChunk, error),
	file WriteFileMetadata,
	firstData []byte,
	firstFinal bool,
	maxBytes int64,
	sink WriteChunkSink,
) error {
	totalBytes := int64(0)
	processOne := func(data []byte, final bool) error {
		if err := ValidateChunkSize(data, maxBytes); err != nil {
			return err
		}
		totalBytes += int64(len(data))
		finished, err := sink(data, final, totalBytes)
		if err != nil {
			return err
		}
		if !final {
			return nil
		}
		_ = finished
		if _, err := recv(); !errors.Is(err, io.EOF) {
			if err != nil {
				return err
			}
			return status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		return nil
	}
	if err := processOne(firstData, firstFinal); err != nil || firstFinal {
		return err
	}
	for {
		chunk, err := recv()
		if errors.Is(err, io.EOF) {
			return status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
		}
		if err != nil {
			return err
		}
		if err := ValidateWriteChunkMetadata(file, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
			return err
		}
		if err := processOne(chunk.GetData(), chunk.GetFinalChunk()); err != nil || chunk.GetFinalChunk() {
			return err
		}
	}
}

// ProcessAppendChunks reads append chunks from recv and dispatches them to sink.
func ProcessAppendChunks(
	recv func() (*sharedv1.AppendFileChunk, error),
	file AppendFileMetadata,
	firstData []byte,
	firstFinal bool,
	maxBytes int64,
	sink AppendChunkSink,
) error {
	totalBytes := int64(0)
	processOne := func(data []byte, final bool) error {
		if err := ValidateChunkSize(data, maxBytes); err != nil {
			return err
		}
		totalBytes += int64(len(data))
		finished, err := sink(data, final, totalBytes)
		if err != nil {
			return err
		}
		if !final {
			return nil
		}
		_ = finished
		if _, err := recv(); !errors.Is(err, io.EOF) {
			if err != nil {
				return err
			}
			return status.Error(codes.InvalidArgument, "received chunk after final_chunk")
		}
		return nil
	}
	if err := processOne(firstData, firstFinal); err != nil || firstFinal {
		return err
	}
	for {
		chunk, err := recv()
		if errors.Is(err, io.EOF) {
			return status.Error(codes.InvalidArgument, "final_chunk must be set on the last chunk")
		}
		if err != nil {
			return err
		}
		if err := ValidateWriteChunkMetadata(file.WriteFileMetadata, chunk.GetObjectId(), chunk.GetFilename(), chunk.GetExpectedSize()); err != nil {
			return err
		}
		if chunk.GetCurrentExpectedSize() != file.CurrentExpectedSize {
			return status.Error(codes.InvalidArgument, "current_expected_size must match across all chunks")
		}
		if err := processOne(chunk.GetData(), chunk.GetFinalChunk()); err != nil || chunk.GetFinalChunk() {
			return err
		}
	}
}
