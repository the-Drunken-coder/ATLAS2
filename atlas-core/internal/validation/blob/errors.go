package blob

import (
	"errors"
	"fmt"

	"github.com/anomalyco/atlas-core/internal/core/model"
)

type Violation struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationError struct {
	Violations []Violation `json:"violations"`
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.Violations) == 0 {
		return model.ErrInvalidInput.Error()
	}
	first := e.Violations[0]
	return fmt.Sprintf("%s: %s (field: %s)", first.Code, first.Message, first.Field)
}

func (e *ValidationError) Is(target error) bool {
	return errors.Is(target, model.ErrInvalidInput)
}

func newValidationError(violations []Violation) error {
	if len(violations) == 0 {
		return nil
	}
	return &ValidationError{Violations: violations}
}
