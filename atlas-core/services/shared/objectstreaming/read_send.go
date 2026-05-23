package objectstreaming

import (
	"errors"
	"fmt"
	"io"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
)

// SendObjectFileChunks reads from reader and sends FileChunk messages until totalSize bytes are sent.
func SendObjectFileChunks(reader io.Reader, totalSize, chunkSize int64, send func(*sharedv1.FileChunk) error) error {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	} else if chunkSize > DefaultChunkSize {
		chunkSize = DefaultChunkSize
	}
	if totalSize == 0 {
		return send(&sharedv1.FileChunk{FinalChunk: true, TotalSize: 0})
	}
	buffer := make([]byte, chunkSize)
	sentBytes := int64(0)
	isFirstChunk := true
	for sentBytes < totalSize {
		remaining := totalSize - sentBytes
		readBuf := buffer
		if remaining < int64(len(readBuf)) {
			readBuf = readBuf[:int(remaining)]
		}
		n, err := reader.Read(readBuf)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if n == 0 {
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}
		chunk := &sharedv1.FileChunk{
			Data: append([]byte(nil), readBuf[:n]...),
		}
		sentBytes += int64(n)
		if isFirstChunk {
			chunk.TotalSize = totalSize
			isFirstChunk = false
		}
		chunk.FinalChunk = sentBytes == totalSize
		if err := send(chunk); err != nil {
			return err
		}
		if chunk.FinalChunk {
			return nil
		}
	}
	return fmt.Errorf("object file stream truncated: sent %d of %d bytes", sentBytes, totalSize)
}
