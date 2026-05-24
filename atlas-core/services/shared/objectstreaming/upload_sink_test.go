package objectstreaming

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewForwardWriteSinkSendsFinalChunkOnly(t *testing.T) {
	var sendCalls int
	var sentFinal bool
	sink := NewForwardWriteSink(2, func(data []byte, final bool) error {
		sendCalls++
		sentFinal = final
		return nil
	})

	if err := sink([]byte("ok"), true, 2); err != nil {
		t.Fatalf("sink: %v", err)
	}
	if !sentFinal {
		t.Fatal("expected final chunk send")
	}
	if sendCalls != 1 {
		t.Fatalf("expected single final send, got %d", sendCalls)
	}
}

func TestNewForwardWriteSinkRejectsSizeMismatch(t *testing.T) {
	sink := NewForwardWriteSink(10, func([]byte, bool) error { return nil })
	if err := sink(nil, true, 2); err == nil || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestNewForwardAppendSinkSendsFinalChunkOnly(t *testing.T) {
	var sendCalls int
	var sentFinal bool
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ExpectedSize: 2},
		CurrentExpectedSize: 0,
	}
	sink := NewForwardAppendSink(file, func(data []byte, final bool) error {
		sendCalls++
		sentFinal = final
		return nil
	})

	if err := sink([]byte("ok"), true, 2); err != nil {
		t.Fatalf("sink: %v", err)
	}
	if !sentFinal {
		t.Fatal("expected final chunk send")
	}
	if sendCalls != 1 {
		t.Fatalf("expected single final send, got %d", sendCalls)
	}
}

func TestNewForwardAppendSinkRejectsSizeMismatch(t *testing.T) {
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ExpectedSize: 10},
		CurrentExpectedSize: 0,
	}
	sink := NewForwardAppendSink(file, func([]byte, bool) error { return nil })
	if err := sink(nil, true, 2); err == nil || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}
