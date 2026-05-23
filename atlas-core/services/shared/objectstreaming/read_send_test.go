package objectstreaming

import (
	"bytes"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
)

func TestSendObjectFileChunksCapsReadToTotalSize(t *testing.T) {
	// Reader has more bytes than totalSize; without capping, extra bytes would be sent.
	data := []byte("0123456789")
	var chunks []*sharedv1.FileChunk
	var sent int64
	if err := SendObjectFileChunks(bytes.NewReader(data), 6, 4, func(chunk *sharedv1.FileChunk) error {
		chunks = append(chunks, chunk)
		sent += int64(len(chunk.GetData()))
		return nil
	}); err != nil {
		t.Fatalf("SendObjectFileChunks: %v", err)
	}
	if sent != 6 {
		t.Fatalf("sent %d bytes, want 6", sent)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].GetTotalSize() != 6 {
		t.Fatalf("first chunk total_size = %d, want 6", chunks[0].GetTotalSize())
	}
	if !chunks[1].GetFinalChunk() {
		t.Fatal("expected final chunk")
	}
	if string(chunks[0].GetData())+string(chunks[1].GetData()) != "012345" {
		t.Fatalf("chunk data = %q, want %q", string(chunks[0].GetData())+string(chunks[1].GetData()), "012345")
	}
}

func TestSendObjectFileChunksTruncatedReader(t *testing.T) {
	err := SendObjectFileChunks(bytes.NewReader([]byte("ab")), 6, 2, func(*sharedv1.FileChunk) error { return nil })
	if err == nil {
		t.Fatal("expected truncation error")
	}
}
