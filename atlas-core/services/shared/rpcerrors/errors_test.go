package rpcerrors

import (
	"context"
	"errors"
	"testing"

	"github.com/anomalyco/atlas-core/services/shared/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatusMapsContextCanceled(t *testing.T) {
	err := ToStatus(context.Canceled)
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status, got %T", err)
	}
	if st.Code() != codes.Canceled {
		t.Fatalf("expected code %s, got %s", codes.Canceled, st.Code())
	}
}

func TestToStatusMapsDeadlineExceeded(t *testing.T) {
	err := ToStatus(context.DeadlineExceeded)
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status, got %T", err)
	}
	if st.Code() != codes.DeadlineExceeded {
		t.Fatalf("expected code %s, got %s", codes.DeadlineExceeded, st.Code())
	}
}

func TestFromStatusMapsContextCodes(t *testing.T) {
	if !errors.Is(FromStatus(status.Error(codes.Canceled, context.Canceled.Error())), context.Canceled) {
		t.Fatal("expected canceled status to map back to context.Canceled")
	}
	if !errors.Is(FromStatus(status.Error(codes.DeadlineExceeded, context.DeadlineExceeded.Error())), context.DeadlineExceeded) {
		t.Fatal("expected deadline-exceeded status to map back to context.DeadlineExceeded")
	}
}

func TestFromStatusPreservesInvalidInputSentinel(t *testing.T) {
	err := FromStatus(status.Error(codes.InvalidArgument, "bad input"))
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("expected invalid-argument status to wrap ErrInvalidInput, got %v", err)
	}
}

func TestFromStatusReturnsBareInvalidInputSentinelForEmptyMessage(t *testing.T) {
	err := FromStatus(status.Error(codes.InvalidArgument, ""))
	if err != model.ErrInvalidInput {
		t.Fatalf("expected bare ErrInvalidInput sentinel, got %v", err)
	}
}
