package rpcerrors

import (
	"context"
	"errors"
	"fmt"

	"atlas.local/protocol"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const internalServerErrorMessage = "internal server error"

func ToStatus(err error) error {
	return ToStatusContext(context.Background(), nil, err)
}

func ToStatusContext(ctx context.Context, log *logging.Logger, err error) error {
	if err == nil {
		return nil
	}
	// Preserve already-classified gRPC status errors so streaming
	// helpers can return codes.ResourceExhausted, codes.FailedPrecondition,
	// etc. without being re-wrapped as codes.Internal.
	if _, ok := status.FromError(err); ok {
		return err
	}
	st := baseStatus(err)
	if log != nil && st.Code() == codes.Internal {
		log.ErrorContext(ctx, "rpcerrors", "unmapped rpc error", logging.ErrorField(err))
	}
	detail := detailFromError(err)
	if detail == nil {
		return st.Err()
	}
	withDetails, detailErr := st.WithDetails(detail)
	if detailErr != nil {
		return st.Err()
	}
	return withDetails.Err()
}

func FromStatus(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	for _, detail := range st.Details() {
		if _, ok := detail.(error); ok {
			return fromCode(st.Code(), st.Message())
		}
		pb, ok := detail.(*sharedv1.ErrorDetail)
		if !ok {
			continue
		}
		if len(pb.GetValidationIssues()) > 0 {
			issues := make([]protocol.ValidationIssue, 0, len(pb.GetValidationIssues()))
			for _, issue := range pb.GetValidationIssues() {
				issues = append(issues, protocol.ValidationIssue{Field: issue.GetField(), Code: issue.GetCode(), Message: issue.GetMessage()})
			}
			return protocolvalidation.NewValidationError(issues)
		}
		if pb.Field != nil {
			return model.NewFieldError(pb.GetCode(), pb.GetMessage(), pb.GetField())
		}
		return model.NewCoreError(pb.GetCode(), pb.GetMessage())
	}
	return fromCode(st.Code(), st.Message())
}

func baseStatus(err error) *status.Status {
	switch {
	case errors.Is(err, model.ErrNotFound):
		return status.New(codes.NotFound, err.Error())
	case errors.Is(err, model.ErrConflict):
		return status.New(codes.AlreadyExists, err.Error())
	case errors.Is(err, model.ErrIdempotencyConflict):
		return status.New(codes.FailedPrecondition, err.Error())
	case errors.Is(err, model.ErrVersionConflict):
		return status.New(codes.Aborted, err.Error())
	case errors.Is(err, model.ErrInvalidInput):
		return status.New(codes.InvalidArgument, err.Error())
	case errors.Is(err, context.Canceled):
		return status.New(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.New(codes.DeadlineExceeded, err.Error())
	default:
		var fieldErr *model.FieldError
		if errors.As(err, &fieldErr) {
			switch fieldErr.Code {
			case "CONFLICT":
				return status.New(codes.AlreadyExists, fieldErr.Error())
			case "IDEMPOTENCY_CONFLICT":
				return status.New(codes.FailedPrecondition, fieldErr.Error())
			default:
				return status.New(codes.InvalidArgument, fieldErr.Error())
			}
		}
		var validationErr *protocolvalidation.ValidationError
		if errors.As(err, &validationErr) {
			return status.New(codes.InvalidArgument, validationErr.Error())
		}
		return status.New(codes.Internal, internalServerErrorMessage)
	}
}

func detailFromError(err error) *sharedv1.ErrorDetail {
	var fieldErr *model.FieldError
	if errors.As(err, &fieldErr) {
		detail := &sharedv1.ErrorDetail{Code: fieldErr.Code, Message: fieldErr.Message}
		if fieldErr.Field != "" {
			detail.Field = &fieldErr.Field
		}
		return detail
	}
	var coreErr *model.CoreError
	if errors.As(err, &coreErr) {
		return &sharedv1.ErrorDetail{Code: coreErr.Code, Message: coreErr.Message}
	}
	var validationErr *protocolvalidation.ValidationError
	if errors.As(err, &validationErr) {
		detail := &sharedv1.ErrorDetail{Code: "VALIDATION_FAILED", Message: validationErr.Error()}
		for _, issue := range validationErr.Issues {
			detail.ValidationIssues = append(detail.ValidationIssues, &sharedv1.ValidationIssue{Field: issue.Field, Code: issue.Code, Message: issue.Message})
		}
		return detail
	}
	return nil
}

func fromCode(code codes.Code, message string) error {
	switch code {
	case codes.NotFound:
		return model.ErrNotFound
	case codes.AlreadyExists:
		return model.ErrConflict
	case codes.FailedPrecondition:
		if message == "" {
			return model.ErrIdempotencyConflict
		}
		return fmt.Errorf("%w: %s", model.ErrIdempotencyConflict, message)
	case codes.Aborted:
		return model.ErrVersionConflict
	case codes.InvalidArgument:
		if message == "" {
			return model.ErrInvalidInput
		}
		return fmt.Errorf("%w: %s", model.ErrInvalidInput, message)
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return fmt.Errorf("%s", message)
	}
}
