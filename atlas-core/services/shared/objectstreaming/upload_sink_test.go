package objectstreaming

import (
	"io"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewForwardWriteSinkDrainsRecvAndFinishes(t *testing.T) {
	recvCalls := 0
	recv := func() (*sharedv1.WriteFileChunk, error) {
		recvCalls++
		return nil, io.EOF
	}
	var sentFinal bool
	sink := NewForwardWriteSink(2, recv, func(data []byte, final bool) error {
		sentFinal = final
		return nil
	}, func() error { return nil })

	finished, err := sink([]byte("ok"), true, 2)
	if err != nil {
		t.Fatalf("sink: %v", err)
	}
	if !finished {
		t.Fatal("expected finished=true")
	}
	if !sentFinal {
		t.Fatal("expected final chunk send")
	}
	if recvCalls != 1 {
		t.Fatalf("expected one trailing recv, got %d", recvCalls)
	}
}

func TestNewForwardWriteSinkRejectsSizeMismatch(t *testing.T) {
	sink := NewForwardWriteSink(10, func() (*sharedv1.WriteFileChunk, error) { return nil, io.EOF },
		func([]byte, bool) error { return nil },
		func() error { return nil },
	)
	_, err := sink(nil, true, 2)
	if err == nil || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestNewForwardAppendSinkDrainsRecvAndFinishes(t *testing.T) {
	recvCalls := 0
	recv := func() (*sharedv1.AppendFileChunk, error) {
		recvCalls++
		return nil, io.EOF
	}
	var sentFinal bool
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ExpectedSize: 2},
		CurrentExpectedSize: 0,
	}
	sink := NewForwardAppendSink(file, recv, func(data []byte, final bool) error {
		sentFinal = final
		return nil
	}, func() error { return nil })

	finished, err := sink([]byte("ok"), true, 2)
	if err != nil {
		t.Fatalf("sink: %v", err)
	}
	if !finished {
		t.Fatal("expected finished=true")
	}
	if !sentFinal {
		t.Fatal("expected final chunk send")
	}
	if recvCalls != 1 {
		t.Fatalf("expected one trailing recv, got %d", recvCalls)
	}
}

func TestNewForwardAppendSinkRejectsSizeMismatch(t *testing.T) {
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ExpectedSize: 10},
		CurrentExpectedSize: 0,
	}
	sink := NewForwardAppendSink(file, func() (*sharedv1.AppendFileChunk, error) { return nil, io.EOF },
		func([]byte, bool) error { return nil },
		func() error { return nil },
	)
	_, err := sink(nil, true, 2)
	if err == nil || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}
