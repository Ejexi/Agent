package types

import (
	"fmt"
	"time"
)

type ErrorCode string

// error codes used in the project
const (
	// General errors (1xxx)
	ErrCodeInternal     ErrorCode = "ERR_1000"
	ErrCodeNotFound     ErrorCode = "ERR_1001"
	ErrCodeInvalidInput ErrorCode = "ERR_1002"

	// Agent errors (2xxx)
	ErrCodeAgentFailed ErrorCode = "ERR_2001"

	// Tool errors (3xxx)
	ErrCodeToolNotFound   ErrorCode = "ERR_3000"
	ErrCodeToolExecution  ErrorCode = "ERR_3001"
	ErrCodeToolValidation ErrorCode = "ERR_3002"

	// Security errors (4xxx)
	ErrCodeAuthFailed       ErrorCode = "ERR_4000"
	ErrCodePermissionDenied ErrorCode = "ERR_4003"
)

type AppError struct {
	Code      ErrorCode
	Message   string
	Cause     error                  // The original error
	Timestamp time.Time              // When the error occurred
	Context   map[string]interface{} // Additional context
}

// Error implements the error interface
// This is required for AppError to be considered an "error" in Go
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the cause (for error wrapping)
func (e *AppError) Unwrap() error {
	return e.Cause
}

// creates a new AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// wraps an existing error with context
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// WithContext adds context to an error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	e.Context[key] = value
	return e
}
