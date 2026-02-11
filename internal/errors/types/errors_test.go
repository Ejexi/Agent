package types

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(ErrCodeNotFound, "User not found")

	if err.Code != ErrCodeNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeNotFound, err.Code)
	}
	if err.Message != "User not found" {
		t.Errorf("Expected message 'User not found', got %s", err.Message)
	}
	// Check that Error() method works
	errStr := err.Error()
	if errStr == "" {
		t.Error("Error string should not be empty")
	}
}
func TestWrap(t *testing.T) {
	originalErr := errors.New("database connection failed")
	wrappedErr := Wrap(originalErr, ErrCodeInternal, "Failed to connect")

	if wrappedErr.Cause != originalErr {
		t.Error("Cause should be the original error")
	}
	// Test Unwrap
	unwrapped := wrappedErr.Unwrap()
	if unwrapped != originalErr {
		t.Error("Unwrap should return original error")
	}
}

func TestWithContext(t *testing.T) {
	err := New(ErrCodeToolExecution, "Tool failed")
	err.WithContext("tool_name", "security-scanner")
	err.WithContext("duration_ms", 1500)

	if err.Context["tool_name"] != "security-scanner" {
		t.Error("Context should contain tool_name")
	}
	if err.Context["duration_ms"] != 1500 {
		t.Error("Context should contain duration_ms")
	}
}
