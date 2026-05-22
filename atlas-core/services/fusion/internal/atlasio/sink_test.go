package atlasio

import (
	"io"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestReadStreamedFileBytesTreatsOpenNotFoundAsEmpty(t *testing.T) {
	data, err := readStreamedFileBytes("obj_001", func() (*sharedv1.FileChunk, error) {
		return nil, status.Error(codes.NotFound, "missing file")
	})
	if err != nil {
		t.Fatalf("expected empty provenance, got error: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected no data, got %q", data)
	}
}

func TestReadStreamedFileBytesFailsOnMidStreamNotFound(t *testing.T) {
	calls := 0
	_, err := readStreamedFileBytes("obj_001", func() (*sharedv1.FileChunk, error) {
		calls++
		if calls == 1 {
			return &sharedv1.FileChunk{Data: []byte("line1\n")}, nil
		}
		return nil, status.Error(codes.NotFound, "stream reset")
	})
	if err == nil {
		t.Fatal("expected error when NotFound after data was received")
	}
}

func TestReadStreamedFileBytesReadsUntilEOF(t *testing.T) {
	calls := 0
	data, err := readStreamedFileBytes("obj_001", func() (*sharedv1.FileChunk, error) {
		calls++
		if calls == 1 {
			return &sharedv1.FileChunk{Data: []byte("abc")}, nil
		}
		return nil, io.EOF
	})
	if err != nil {
		t.Fatalf("readStreamedFileBytes: %v", err)
	}
	if string(data) != "abc" {
		t.Fatalf("expected abc, got %q", data)
	}
}
