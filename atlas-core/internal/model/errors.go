package model

import "fmt"

type CoreError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *CoreError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *CoreError) Is(target error) bool {
	other, ok := target.(*CoreError)
	return ok && e.Code != "" && e.Code == other.Code
}

func NewCoreError(code, message string) *CoreError {
	return &CoreError{Code: code, Message: message}
}

type FieldError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field"`
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s: %s (field: %s)", e.Code, e.Message, e.Field)
}

func (e *FieldError) Is(target error) bool {
	if e == nil || e.Code == "" {
		return false
	}
	switch other := target.(type) {
	case *CoreError:
		return other != nil && other.Code != "" && e.Code == other.Code
	case *FieldError:
		return other != nil && other.Code != "" && e.Code == other.Code
	default:
		return false
	}
}

func NewFieldError(code, message, field string) *FieldError {
	return &FieldError{Code: code, Message: message, Field: field}
}

var (
	ErrNotFound        = NewCoreError("NOT_FOUND", "resource not found")
	ErrConflict        = NewCoreError("CONFLICT", "resource conflict")
	ErrVersionConflict = NewCoreError("VERSION_CONFLICT", "resource version conflict")
	ErrInternal        = NewCoreError("INTERNAL", "internal error")
	ErrInvalidInput    = NewCoreError("INVALID_INPUT", "invalid input")
	ErrDatabaseError   = NewCoreError("DATABASE_ERROR", "database operation failed")
	ErrStorageError    = NewCoreError("STORAGE_ERROR", "object storage operation failed")
	ErrSchemaError     = NewCoreError("SCHEMA_ERROR", "schema setup failed")
	ErrConfigError     = NewCoreError("CONFIG_ERROR", "configuration error")
)
