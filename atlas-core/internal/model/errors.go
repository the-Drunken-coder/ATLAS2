package model

import "fmt"

type CoreError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *CoreError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
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

func NewFieldError(code, message, field string) *FieldError {
	return &FieldError{Code: code, Message: message, Field: field}
}

var (
	ErrNotFound         = NewCoreError("NOT_FOUND", "resource not found")
	ErrConflict         = NewCoreError("CONFLICT", "resource conflict")
	ErrInternal         = NewCoreError("INTERNAL", "internal error")
	ErrInvalidInput     = NewCoreError("INVALID_INPUT", "invalid input")
	ErrDatabaseError    = NewCoreError("DATABASE_ERROR", "database operation failed")
	ErrStorageError     = NewCoreError("STORAGE_ERROR", "object storage operation failed")
	ErrSchemaError      = NewCoreError("SCHEMA_ERROR", "schema setup failed")
	ErrConfigError      = NewCoreError("CONFIG_ERROR", "configuration error")
)
