package service

import (
	"context"
	"errors"
	"io"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type writeChunkTestStream struct {
	ctx    context.Context
	chunks []*sharedv1.WriteFileChunk
	err    error
	index  int
}

func (s *writeChunkTestStream) Context() context.Context { return s.ctx }

func (s *writeChunkTestStream) Recv() (*sharedv1.WriteFileChunk, error) {
	if s.index < len(s.chunks) {
		chunk := s.chunks[s.index]
		s.index++
		return chunk, nil
	}
	if s.err != nil {
		err := s.err
		s.err = nil
		return nil, err
	}
	return nil, errors.New("unexpected extra recv")
}

type appendChunkTestStream struct {
	ctx    context.Context
	chunks []*sharedv1.AppendFileChunk
	err    error
	index  int
}

func (s *appendChunkTestStream) Context() context.Context { return s.ctx }

func (s *appendChunkTestStream) Recv() (*sharedv1.AppendFileChunk, error) {
	if s.index < len(s.chunks) {
		chunk := s.chunks[s.index]
		s.index++
		return chunk, nil
	}
	if s.err != nil {
		err := s.err
		s.err = nil
		return nil, err
	}
	return nil, errors.New("unexpected extra recv")
}

func TestReceiveWriteObjectFileChunksAllowsEmptyFile(t *testing.T) {
	file, err := receiveWriteObjectFileChunks(&writeChunkTestStream{
		ctx: context.Background(),
		chunks: []*sharedv1.WriteFileChunk{{
			ObjectId:   "obj_001",
			Filename:   "empty.txt",
			FinalChunk: true,
		}},
		err: io.EOF,
	})
	if err != nil {
		t.Fatalf("receive write chunks: %v", err)
	}
	if file.objectID != "obj_001" || file.filename != "empty.txt" {
		t.Fatalf("unexpected metadata: %+v", file)
	}
	if len(file.data) != 0 {
		t.Fatalf("expected empty file, got %d bytes", len(file.data))
	}
}

func TestApplyWriteChunkRejectsOversizedFile(t *testing.T) {
	file := &receivedWriteFile{}
	totalBytes := int64(0)
	err := applyWriteChunk(file, &sharedv1.WriteFileChunk{
		ObjectId:   "obj_001",
		Filename:   "big.bin",
		Data:       []byte("12345"),
		FinalChunk: true,
	}, false, &totalBytes, 4)
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", err)
	}
}

func TestReceiveAppendObjectFileChunksValidatesCurrentExpectedSize(t *testing.T) {
	file, err := receiveAppendObjectFileChunks(&appendChunkTestStream{
		ctx: context.Background(),
		chunks: []*sharedv1.AppendFileChunk{{
			ObjectId:            "obj_001",
			Filename:            "log.txt",
			Data:                []byte("abc"),
			FinalChunk:          true,
			CurrentExpectedSize: 5,
			ExpectedSize:        8,
		}},
		err: io.EOF,
	}, func(context.Context, string, string) (int64, error) {
		return 5, nil
	})
	if err != nil {
		t.Fatalf("receive append chunks: %v", err)
	}
	if got := string(file.data); got != "abc" {
		t.Fatalf("expected appended data abc, got %q", got)
	}
}

func TestReceiveAppendObjectFileChunksRejectsWrongCurrentExpectedSize(t *testing.T) {
	_, err := receiveAppendObjectFileChunks(&appendChunkTestStream{
		ctx: context.Background(),
		chunks: []*sharedv1.AppendFileChunk{{
			ObjectId:            "obj_001",
			Filename:            "log.txt",
			Data:                []byte("abc"),
			FinalChunk:          true,
			CurrentExpectedSize: 4,
			ExpectedSize:        7,
		}},
	}, func(context.Context, string, string) (int64, error) {
		return 5, nil
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func TestReceiveWriteObjectFileChunksPropagatesClientDisconnect(t *testing.T) {
	_, err := receiveWriteObjectFileChunks(&writeChunkTestStream{
		ctx: context.Background(),
		chunks: []*sharedv1.WriteFileChunk{{
			ObjectId: "obj_001",
			Filename: "partial.txt",
			Data:     []byte("partial"),
		}},
		err: context.Canceled,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestSendObjectFileChunksUsesFinalChunkAndTotalSize(t *testing.T) {
	var chunks []*sharedv1.FileChunk
	if err := sendObjectFileChunks([]byte("abcdef"), 2, func(chunk *sharedv1.FileChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}); err != nil {
		t.Fatalf("send chunks: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].GetTotalSize() != 6 {
		t.Fatalf("expected first chunk total_size 6, got %d", chunks[0].GetTotalSize())
	}
	if chunks[2].GetFinalChunk() != true {
		t.Fatal("expected last chunk to be final")
	}
}
