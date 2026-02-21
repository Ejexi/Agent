package types

import (
	"errors"
	"fmt"
	"time"
)

type ErrorCode string

const (

	// General
	ErrCodeInternal     ErrorCode = "ERR_1000"
	ErrCodeNotFound     ErrorCode = "ERR_1001"
	ErrCodeInvalidInput ErrorCode = "ERR_1002"

	// Agent
	
	ErrCodeAgentFailed ErrorCode = "ERR_2001"

	// Tool
	ErrCodeToolNotFound   ErrorCode = "ERR_3000"
	ErrCodeToolExecution  ErrorCode = "ERR_3001"
	ErrCodeToolValidation ErrorCode = "ERR_3002"

	// Security
	ErrCodeAuthFailed       ErrorCode = "ERR_4000"
	ErrCodePermissionDenied ErrorCode = "ERR_4003"
)

type AppError struct {

	Code ErrorCode `json:"code"`

	Message string `json:"message"`

	Cause error `json:"-"`

	Timestamp time.Time `json:"timestamp"`

	Context map[string]interface{} `json:"context,omitempty"`
}

func (e *AppError) Error() string {

	if e.Cause != nil {

		return fmt.Sprintf("[%s] %s: %v",
			e.Code,
			e.Message,
			e.Cause,
		)
	}

	return fmt.Sprintf("[%s] %s",
		e.Code,
		e.Message,
	)
}

func (e *AppError) Unwrap() error {

	return e.Cause
}

// New basic error
func New(
	code ErrorCode,
	message string,
) *AppError {

	return &AppError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Context:   map[string]interface{}{},
	}
}

// Newf formatted error
func Newf(
	code ErrorCode,
	format string,
	args ...interface{},
) *AppError {

	return &AppError{
		Code:      code,
		Message:   fmt.Sprintf(format, args...),
		Timestamp: time.Now(),
		Context:   map[string]interface{}{},
	}
}

// Wrap existing error
func Wrap(
	err error,
	code ErrorCode,
	message string,
) *AppError {

	return &AppError{
		Code:      code,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
		Context:   map[string]interface{}{},
	}
}

// Wrapf formatted wrap
func Wrapf(
	err error,
	code ErrorCode,
	format string,
	args ...interface{},
) *AppError {

	return &AppError{
		Code:      code,
		Message:   fmt.Sprintf(format, args...),
		Cause:     err,
		Timestamp: time.Now(),
		Context:   map[string]interface{}{},
	}
}

// Add context
func (e *AppError) WithContext(
	key string,
	value interface{},
) *AppError {

	e.Context[key] = value

	return e
}

// Add cause after creation
func (e *AppError) WithCause(
	err error,
) *AppError {

	e.Cause = err

	return e
}

// errors.Is support
func Is(err error, target error) bool {

	return errors.Is(err, target)
}

// errors.As support
func As(err error, target interface{}) bool {

	return errors.As(err, target)
}

// Convert to JSON map
func (e *AppError) ToMap() map[string]interface{} {

	return map[string]interface{}{
		"code":      e.Code,
		"message":   e.Message,
		"timestamp": e.Timestamp,
		"context":   e.Context,
	}
}