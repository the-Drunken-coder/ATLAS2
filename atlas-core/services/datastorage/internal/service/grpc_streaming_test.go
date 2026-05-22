package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/objectstreaming"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testMaxObjectFileBytes = 4

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

func TestReceiveFirstWriteChunkAllowsEmptyFile(t *testing.T) {
	chunk, file, err := objectstreaming.ReceiveFirstWriteChunk(&writeChunkTestStream{
		ctx: context.Background(),
		chunks: []*sharedv1.WriteFileChunk{{
			ObjectId:   "obj_001",
			Filename:   "empty.txt",
			FinalChunk: true,
		}},
	})
	if err != nil {
		t.Fatalf("receive write chunk: %v", err)
	}
	if file.ObjectID != "obj_001" || file.Filename != "empty.txt" {
		t.Fatalf("unexpected metadata: %+v", file)
	}
	if !chunk.GetFinalChunk() || len(chunk.GetData()) != 0 {
		t.Fatalf("unexpected first chunk: %+v", chunk)
	}
}

func TestWriteIncomingChunksRejectsOversizedChunk(t *testing.T) {
	stream := &writeChunkTestStream{
		ctx: context.Background(),
		err: io.EOF,
	}
	file := objectstreaming.WriteFileMetadata{ObjectID: "obj_001", Filename: "big.bin", ExpectedSize: 5}
	err := objectstreaming.ProcessWriteChunks(stream.Recv, file, []byte("12345"), true, testMaxObjectFileBytes, objectstreaming.NewWriterSink(io.Discard, file.ExpectedSize))
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", err)
	}
}

func TestWriteIncomingChunksAllowsMultipleChunksOverPreviousCumulativeLimit(t *testing.T) {
	stream := &writeChunkTestStream{
		ctx: context.Background(),
		chunks: []*sharedv1.WriteFileChunk{{
			ObjectId:     "obj_001",
			Filename:     "big.bin",
			ExpectedSize: 8,
			Data:         []byte("1234"),
			FinalChunk:   true,
		}},
		err: io.EOF,
	}
	file := objectstreaming.WriteFileMetadata{ObjectID: "obj_001", Filename: "big.bin", ExpectedSize: 8}
	var out bytes.Buffer
	if err := objectstreaming.ProcessWriteChunks(stream.Recv, file, []byte("abcd"), false, testMaxObjectFileBytes, objectstreaming.NewWriterSink(&out, file.ExpectedSize)); err != nil {
		t.Fatalf("expected multi-chunk stream to pass per-chunk limit, got %v", err)
	}
	if got := out.String(); got != "abcd1234" {
		t.Fatalf("expected concatenated data, got %q", got)
	}
}

func TestReceiveFirstAppendChunkReadsMetadata(t *testing.T) {
	chunk, file, err := objectstreaming.ReceiveFirstAppendChunk(&appendChunkTestStream{
		ctx: context.Background(),
		chunks: []*sharedv1.AppendFileChunk{{
			ObjectId:            "obj_001",
			Filename:            "log.txt",
			Data:                []byte("abc"),
			FinalChunk:          true,
			CurrentExpectedSize: 5,
			ExpectedSize:        8,
		}},
	})
	if err != nil {
		t.Fatalf("receive append chunk: %v", err)
	}
	if file.CurrentExpectedSize != 5 || file.ExpectedSize != 8 {
		t.Fatalf("unexpected append metadata: %+v", file)
	}
	if got := string(chunk.GetData()); got != "abc" {
		t.Fatalf("expected first append chunk abc, got %q", got)
	}
}

func TestWriteIncomingChunksPropagatesClientDisconnect(t *testing.T) {
	stream := &writeChunkTestStream{
		ctx: context.Background(),
		err: context.Canceled,
	}
	file := objectstreaming.WriteFileMetadata{ObjectID: "obj_001", Filename: "partial.txt"}
	var out bytes.Buffer
	err := objectstreaming.ProcessWriteChunks(stream.Recv, file, []byte("partial"), false, objectstreaming.MaxChunkPayloadBytes, objectstreaming.NewWriterSink(&out, file.ExpectedSize))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestAppendIncomingChunksRejectsOversizedChunk(t *testing.T) {
	stream := &appendChunkTestStream{
		ctx: context.Background(),
		err: io.EOF,
	}
	file := objectstreaming.AppendFileMetadata{WriteFileMetadata: objectstreaming.WriteFileMetadata{ObjectID: "obj_001", Filename: "big.bin", ExpectedSize: 9}, CurrentExpectedSize: 4}
	err := objectstreaming.ProcessAppendChunks(stream.Recv, file, []byte("12345"), true, testMaxObjectFileBytes, objectstreaming.NewAppendWriterSink(io.Discard, 4, file.ExpectedSize))
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", err)
	}
}

func TestAppendIncomingChunksAllowsLargeTotalFileAcrossSmallChunks(t *testing.T) {
	stream := &appendChunkTestStream{
		ctx: context.Background(),
		chunks: []*sharedv1.AppendFileChunk{{
			ObjectId:            "obj_001",
			Filename:            "big.bin",
			CurrentExpectedSize: 4,
			ExpectedSize:        12,
			Data:                []byte("1234"),
			FinalChunk:          true,
		}},
		err: io.EOF,
	}
	file := objectstreaming.AppendFileMetadata{
		WriteFileMetadata:   objectstreaming.WriteFileMetadata{ObjectID: "obj_001", Filename: "big.bin", ExpectedSize: 12},
		CurrentExpectedSize: 4,
	}
	var out bytes.Buffer
	if err := objectstreaming.ProcessAppendChunks(stream.Recv, file, []byte("abcd"), false, testMaxObjectFileBytes, objectstreaming.NewAppendWriterSink(&out, 4, file.ExpectedSize)); err != nil {
		t.Fatalf("expected append stream to enforce per-chunk limit only, got %v", err)
	}
	if got := out.String(); got != "abcd1234" {
		t.Fatalf("expected appended data, got %q", got)
	}
}

func TestSendObjectFileChunksUsesFinalChunkAndTotalSize(t *testing.T) {
	var chunks []*sharedv1.FileChunk
	if err := sendObjectFileChunks(bytes.NewReader([]byte("abcdef")), 6, 2, func(chunk *sharedv1.FileChunk) error {
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
	if !chunks[2].GetFinalChunk() {
		t.Fatal("expected last chunk to be final")
	}
}

func TestSendObjectFileChunksClampsOversizedChunkRequests(t *testing.T) {
	var chunks []*sharedv1.FileChunk
	data := bytes.Repeat([]byte("a"), objectstreaming.DefaultChunkSize+10)
	if err := sendObjectFileChunks(bytes.NewReader(data), int64(len(data)), objectstreaming.DefaultChunkSize*2, func(chunk *sharedv1.FileChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}); err != nil {
		t.Fatalf("send chunks: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks after clamping, got %d", len(chunks))
	}
	if got := len(chunks[0].GetData()); got != objectstreaming.DefaultChunkSize {
		t.Fatalf("expected first chunk size %d, got %d", objectstreaming.DefaultChunkSize, got)
	}
}
