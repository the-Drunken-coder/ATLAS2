package rpcerrors

import (
	"context"
	"errors"
	"strings"
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

func TestToStatusMapsIdempotencyConflictToFailedPrecondition(t *testing.T) {
	err := ToStatus(model.NewIdempotencyKeyConflictError("key-1", "obj_other"))
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status, got %T", err)
	}
	if st.Code() != codes.FailedPrecondition {
		t.Fatalf("expected code %s, got %s", codes.FailedPrecondition, st.Code())
	}
}

func TestToStatusMapsDuplicateConflictToAlreadyExists(t *testing.T) {
	err := ToStatus(model.ErrConflict)
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status, got %T", err)
	}
	if st.Code() != codes.AlreadyExists {
		t.Fatalf("expected code %s, got %s", codes.AlreadyExists, st.Code())
	}
}

func TestFromStatusMapsIdempotencyFailedPrecondition(t *testing.T) {
	err := FromStatus(status.Error(codes.FailedPrecondition, "idempotency key conflict"))
	if !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency sentinel, got %v", err)
	}
}

func TestFromStatusDoesNotMapAppendPreconditionToIdempotency(t *testing.T) {
	msg := "append precondition failed: actual size 1 does not match expected size 2"
	err := FromStatus(status.Error(codes.FailedPrecondition, msg))
	if errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("expected append precondition not to map to idempotency, got %v", err)
	}
	if err.Error() != msg {
		t.Fatalf("expected message %q, got %q", msg, err.Error())
	}
}

func TestFromStatusDoesNotMapEmptyFailedPreconditionToIdempotency(t *testing.T) {
	err := FromStatus(status.Error(codes.FailedPrecondition, ""))
	if errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("expected empty precondition not to map to idempotency, got %v", err)
	}
}

func TestToStatusRedactsInternalMessage(t *testing.T) {
	err := ToStatus(errors.New("secret production detail"))
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status, got %T", err)
	}
	if st.Code() != codes.Internal {
		t.Fatalf("expected code %s, got %s", codes.Internal, st.Code())
	}
	if strings.Contains(st.Message(), "secret production detail") {
		t.Fatalf("expected internal status message to be redacted, got %q", st.Message())
	}
	if st.Message() != internalServerErrorMessage {
		t.Fatalf("expected redacted message %q, got %q", internalServerErrorMessage, st.Message())
	}
}
