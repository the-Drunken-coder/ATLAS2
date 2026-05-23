package objectstreaming

import (
	"errors"
	"io"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProcessWriteChunksRejectsChunksAfterFinal(t *testing.T) {
	file := WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2}
	recvCalls := 0
	recv := func() (*sharedv1.WriteFileChunk, error) {
		recvCalls++
		return &sharedv1.WriteFileChunk{
			ObjectId: "obj_001",
			Filename: "data.bin",
			Data:     []byte("late"),
		}, nil
	}
	sink := WriteChunkSink(func(data []byte, final bool, totalBytes int64) error {
		return nil
	})
	err := ProcessWriteChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink)
	if err == nil {
		t.Fatal("expected error for chunk after final_chunk")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if recvCalls != 1 {
		t.Fatalf("expected one post-final recv, got %d", recvCalls)
	}
}

func TestProcessWriteChunksAllowsEOFAfterFinal(t *testing.T) {
	file := WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2}
	recvCalls := 0
	recv := func() (*sharedv1.WriteFileChunk, error) {
		recvCalls++
		return nil, io.EOF
	}
	sink := WriteChunkSink(func(data []byte, final bool, totalBytes int64) error {
		return nil
	})
	if err := ProcessWriteChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink); err != nil {
		t.Fatalf("expected success when trailing recv is EOF, got %v", err)
	}
	if recvCalls != 1 {
		t.Fatalf("expected one post-final recv, got %d", recvCalls)
	}
}

func TestProcessWriteChunksPropagatesRecvErrorAfterFinal(t *testing.T) {
	file := WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2}
	recv := func() (*sharedv1.WriteFileChunk, error) {
		return nil, errors.New("stream broken")
	}
	sink := WriteChunkSink(func(data []byte, final bool, totalBytes int64) error {
		return nil
	})
	err := ProcessWriteChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink)
	if err == nil || err.Error() != "stream broken" {
		t.Fatalf("expected stream error, got %v", err)
	}
}

func TestProcessWriteChunksForwardSinkSingleTrailingRecv(t *testing.T) {
	file := WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2}
	recvCalls := 0
	recv := func() (*sharedv1.WriteFileChunk, error) {
		recvCalls++
		return nil, io.EOF
	}
	var finishCalls int
	sink := NewForwardWriteSink(file.ExpectedSize, func([]byte, bool) error { return nil }, func() error {
		finishCalls++
		return nil
	})
	if err := ProcessWriteChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink); err != nil {
		t.Fatalf("ProcessWriteChunks: %v", err)
	}
	if recvCalls != 1 {
		t.Fatalf("expected one post-final recv on forward path, got %d", recvCalls)
	}
	if finishCalls != 1 {
		t.Fatalf("expected one finish call, got %d", finishCalls)
	}
}

func TestProcessAppendChunksRejectsChunksAfterFinal(t *testing.T) {
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2},
		CurrentExpectedSize: 2,
	}
	recvCalls := 0
	recv := func() (*sharedv1.AppendFileChunk, error) {
		recvCalls++
		return &sharedv1.AppendFileChunk{
			ObjectId:            "obj_001",
			Filename:            "data.bin",
			Data:                []byte("late"),
			CurrentExpectedSize: 2,
		}, nil
	}
	sink := AppendChunkSink(func(data []byte, final bool, totalBytes int64) error {
		return nil
	})
	err := ProcessAppendChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink)
	if err == nil {
		t.Fatal("expected error for chunk after final_chunk")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if recvCalls != 1 {
		t.Fatalf("expected one post-final recv, got %d", recvCalls)
	}
}

func TestProcessAppendChunksAllowsEOFAfterFinal(t *testing.T) {
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2},
		CurrentExpectedSize: 2,
	}
	recvCalls := 0
	recv := func() (*sharedv1.AppendFileChunk, error) {
		recvCalls++
		return nil, io.EOF
	}
	sink := AppendChunkSink(func(data []byte, final bool, totalBytes int64) error {
		return nil
	})
	if err := ProcessAppendChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink); err != nil {
		t.Fatalf("expected success when trailing recv is EOF, got %v", err)
	}
	if recvCalls != 1 {
		t.Fatalf("expected one post-final recv, got %d", recvCalls)
	}
}

func TestProcessAppendChunksPropagatesRecvErrorAfterFinal(t *testing.T) {
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2},
		CurrentExpectedSize: 2,
	}
	recv := func() (*sharedv1.AppendFileChunk, error) {
		return nil, errors.New("stream broken")
	}
	sink := AppendChunkSink(func(data []byte, final bool, totalBytes int64) error {
		return nil
	})
	err := ProcessAppendChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink)
	if err == nil || err.Error() != "stream broken" {
		t.Fatalf("expected stream error, got %v", err)
	}
}

func TestProcessAppendChunksForwardSinkSingleTrailingRecv(t *testing.T) {
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2},
		CurrentExpectedSize: 0,
	}
	recvCalls := 0
	recv := func() (*sharedv1.AppendFileChunk, error) {
		recvCalls++
		return nil, io.EOF
	}
	var finishCalls int
	sink := NewForwardAppendSink(file, func([]byte, bool) error { return nil }, func() error {
		finishCalls++
		return nil
	})
	if err := ProcessAppendChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink); err != nil {
		t.Fatalf("ProcessAppendChunks: %v", err)
	}
	if recvCalls != 1 {
		t.Fatalf("expected one post-final recv on forward path, got %d", recvCalls)
	}
	if finishCalls != 1 {
		t.Fatalf("expected one finish call, got %d", finishCalls)
	}
}
