package objectstreaming

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewForwardWriteSinkSendsFinalAndFinishes(t *testing.T) {
	var order []string
	var sentFinal bool
	sink := NewForwardWriteSink(2, func(data []byte, final bool) error {
		order = append(order, "send")
		sentFinal = final
		return nil
	}, func() error {
		order = append(order, "finish")
		return nil
	})

	if err := sink([]byte("ok"), true, 2); err != nil {
		t.Fatalf("sink: %v", err)
	}
	if !sentFinal {
		t.Fatal("expected final chunk send")
	}
	if len(order) != 2 || order[0] != "send" || order[1] != "finish" {
		t.Fatalf("expected final send then finish, got order %v", order)
	}
}

func TestNewForwardWriteSinkRejectsSizeMismatch(t *testing.T) {
	sink := NewForwardWriteSink(10,
		func([]byte, bool) error { return nil },
		func() error { return nil },
	)
	if err := sink(nil, true, 2); err == nil || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestNewForwardAppendSinkSendsFinalAndFinishes(t *testing.T) {
	var order []string
	var sentFinal bool
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ExpectedSize: 2},
		CurrentExpectedSize: 0,
	}
	sink := NewForwardAppendSink(file, func(data []byte, final bool) error {
		order = append(order, "send")
		sentFinal = final
		return nil
	}, func() error {
		order = append(order, "finish")
		return nil
	})

	if err := sink([]byte("ok"), true, 2); err != nil {
		t.Fatalf("sink: %v", err)
	}
	if !sentFinal {
		t.Fatal("expected final chunk send")
	}
	if len(order) != 2 || order[0] != "send" || order[1] != "finish" {
		t.Fatalf("expected final send then finish, got order %v", order)
	}
}

func TestNewForwardAppendSinkRejectsSizeMismatch(t *testing.T) {
	file := AppendFileMetadata{
		WriteFileMetadata:   WriteFileMetadata{ExpectedSize: 10},
		CurrentExpectedSize: 0,
	}
	sink := NewForwardAppendSink(file,
		func([]byte, bool) error { return nil },
		func() error { return nil },
	)
	if err := sink(nil, true, 2); err == nil || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}
