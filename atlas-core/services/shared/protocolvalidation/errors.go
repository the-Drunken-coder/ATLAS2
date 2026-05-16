package protocolvalidation

import (
	"fmt"
	"strings"

	"atlas.local/protocol"
)

type ValidationError struct {
	Issues []protocol.ValidationIssue
}

func NewValidationError(issues []protocol.ValidationIssue) *ValidationError {
	return &ValidationError{Issues: issues}
}

func (e *ValidationError) Error() string {
	var parts []string
	for _, issue := range e.Issues {
		parts = append(parts, fmt.Sprintf("%s:%s: %s", issue.Field, issue.Code, issue.Message))
	}
	return "VALIDATION_FAILED: " + strings.Join(parts, "; ")
}
