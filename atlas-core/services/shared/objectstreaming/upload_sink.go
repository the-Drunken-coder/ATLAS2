package objectstreaming

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewForwardWriteSink returns a sink that forwards non-final chunks via sendChunk.
// On the final chunk it sends the final payload and runs finish. ProcessWriteChunks
// performs the single trailing recv/EOF check; this sink must not call recv.
func NewForwardWriteSink(
	expectedSize int64,
	sendChunk func([]byte, bool) error,
	finish func() error,
) WriteChunkSink {
	return func(data []byte, final bool, totalBytes int64) error {
		if !final {
			if err := sendChunk(data, false); err != nil {
				return err
			}
			return nil
		}
		if expectedSize != 0 && totalBytes != expectedSize {
			return status.Error(codes.InvalidArgument, fmt.Sprintf("expected_size mismatch: got %d bytes, expected %d", totalBytes, expectedSize))
		}
		if err := sendChunk(data, true); err != nil {
			return err
		}
		return finish()
	}
}

// NewForwardAppendSink is the append-stream variant of NewForwardWriteSink.
// ProcessAppendChunks performs the single trailing recv/EOF check; this sink must not call recv.
func NewForwardAppendSink(
	file AppendFileMetadata,
	sendChunk func([]byte, bool) error,
	finish func() error,
) AppendChunkSink {
	return func(data []byte, final bool, totalBytes int64) error {
		if !final {
			if err := sendChunk(data, false); err != nil {
				return err
			}
			return nil
		}
		if file.ExpectedSize != 0 && file.CurrentExpectedSize+totalBytes != file.ExpectedSize {
			return status.Error(codes.InvalidArgument, fmt.Sprintf(
				"expected_size mismatch: got %d bytes after append, expected %d",
				file.CurrentExpectedSize+totalBytes, file.ExpectedSize))
		}
		if err := sendChunk(data, true); err != nil {
			return err
		}
		return finish()
	}
}
