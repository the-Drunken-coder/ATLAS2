package objectstreaming

import (
	"errors"
	"io"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProcessWriteChunksRejectsChunksAfterFinalWhenSinkReturnsFinished(t *testing.T) {
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
	sink := WriteChunkSink(func(data []byte, final bool, totalBytes int64) (bool, error) {
		return false, nil
	})
	err := ProcessWriteChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink)
	if err == nil {
		t.Fatal("expected error for chunk after final_chunk")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestProcessWriteChunksAllowsFinishedSinkWhenRecvIsEOF(t *testing.T) {
	file := WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2}
	recv := func() (*sharedv1.WriteFileChunk, error) {
		return nil, io.EOF
	}
	sink := WriteChunkSink(func(data []byte, final bool, totalBytes int64) (bool, error) {
		return true, nil
	})
	if err := ProcessWriteChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink); err != nil {
		t.Fatalf("expected success when trailing recv is EOF, got %v", err)
	}
}

func TestProcessWriteChunksPropagatesRecvErrorAfterFinal(t *testing.T) {
	file := WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2}
	recv := func() (*sharedv1.WriteFileChunk, error) {
		return nil, errors.New("stream broken")
	}
	sink := WriteChunkSink(func(data []byte, final bool, totalBytes int64) (bool, error) {
		return false, nil
	})
	err := ProcessWriteChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink)
	if err == nil || err.Error() != "stream broken" {
		t.Fatalf("expected stream error, got %v", err)
	}
}

func TestProcessAppendChunksRejectsChunksAfterFinalWhenSinkReturnsFinished(t *testing.T) {
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2},
		CurrentExpectedSize: 2,
	}
	recv := func() (*sharedv1.AppendFileChunk, error) {
		return &sharedv1.AppendFileChunk{
			ObjectId:            "obj_001",
			Filename:            "data.bin",
			Data:                []byte("late"),
			CurrentExpectedSize: 2,
		}, nil
	}
	sink := AppendChunkSink(func(data []byte, final bool, totalBytes int64) (bool, error) {
		return false, nil
	})
	err := ProcessAppendChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink)
	if err == nil {
		t.Fatal("expected error for chunk after final_chunk")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestProcessAppendChunksAllowsFinishedSinkWhenRecvIsEOF(t *testing.T) {
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ObjectID: "obj_001", Filename: "data.bin", ExpectedSize: 2},
		CurrentExpectedSize: 2,
	}
	recv := func() (*sharedv1.AppendFileChunk, error) {
		return nil, io.EOF
	}
	sink := AppendChunkSink(func(data []byte, final bool, totalBytes int64) (bool, error) {
		return true, nil
	})
	if err := ProcessAppendChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink); err != nil {
		t.Fatalf("expected success when trailing recv is EOF, got %v", err)
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
	sink := AppendChunkSink(func(data []byte, final bool, totalBytes int64) (bool, error) {
		return false, nil
	})
	err := ProcessAppendChunks(recv, file, []byte("ok"), true, MaxChunkPayloadBytes, sink)
	if err == nil || err.Error() != "stream broken" {
		t.Fatalf("expected stream error, got %v", err)
	}
}
