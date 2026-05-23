package objectstreaming

import (
	"errors"
	"fmt"
	"io"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewForwardWriteSink returns a sink that forwards non-final chunks via sendChunk.
// On the final chunk it drains recv inside the sink (expecting EOF), sends the final payload,
// runs finish, and returns finished=true. ProcessWriteChunks still performs its own trailing
// recv/EOF check after the sink returns.
func NewForwardWriteSink(
	expectedSize int64,
	recv func() (*sharedv1.WriteFileChunk, error),
	sendChunk func([]byte, bool) error,
	finish func() error,
) WriteChunkSink {
	return func(data []byte, final bool, totalBytes int64) (bool, error) {
		if !final {
			if err := sendChunk(data, false); err != nil {
				return false, err
			}
			return false, nil
		}
		if expectedSize != 0 && totalBytes != expectedSize {
			return false, status.Error(codes.InvalidArgument, fmt.Sprintf("expected_size mismatch: got %d bytes, expected %d", totalBytes, expectedSize))
		}
		if err := sendChunk(data, true); err != nil {
			return false, err
		}
		if err := drainFinalRecv(recv); err != nil {
			return false, err
		}
		return true, finish()
	}
}

// NewForwardAppendSink is the append-stream variant of NewForwardWriteSink.
// On the final chunk it drains recv inside the sink; ProcessAppendChunks still performs its
// own trailing recv/EOF check after the sink returns.
func NewForwardAppendSink(
	file AppendFileMetadata,
	recv func() (*sharedv1.AppendFileChunk, error),
	sendChunk func([]byte, bool) error,
	finish func() error,
) AppendChunkSink {
	return func(data []byte, final bool, totalBytes int64) (bool, error) {
		if !final {
			if err := sendChunk(data, false); err != nil {
				return false, err
			}
			return false, nil
		}
		if file.ExpectedSize != 0 && file.CurrentExpectedSize+totalBytes != file.ExpectedSize {
			return false, status.Error(codes.InvalidArgument, fmt.Sprintf(
				"expected_size mismatch: got %d bytes after append, expected %d",
				file.CurrentExpectedSize+totalBytes, file.ExpectedSize))
		}
		if err := sendChunk(data, true); err != nil {
			return false, err
		}
		if err := drainFinalAppendRecv(recv); err != nil {
			return false, err
		}
		return true, finish()
	}
}

func drainFinalRecv(recv func() (*sharedv1.WriteFileChunk, error)) error {
	if _, err := recv(); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return status.Error(codes.InvalidArgument, "received chunk after final_chunk")
	}
	return nil
}

func drainFinalAppendRecv(recv func() (*sharedv1.AppendFileChunk, error)) error {
	if _, err := recv(); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return status.Error(codes.InvalidArgument, "received chunk after final_chunk")
	}
	return nil
}
