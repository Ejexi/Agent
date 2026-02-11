package logger

import (
	"testing"

	"go.uber.org/zap"
)

func TestLogger(t *testing.T) {
	// Try to create a logger
	log, err := New("info")
	// Check if there was an error
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	// Check if logger is not nil (nil = nothing/null)
	if log == nil {
		t.Fatalf("Logger should not be nil")
	}
	log.Info("Test message", zap.String("test", "value"))
}

// tests different log levels
func TestLogLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		log, err := New(level)
		if err != nil {
			t.Errorf("Failed to create logger with level %s: %v", level, err)

		}
		log.Info("Testing level", zap.String("level", level))
	}
}
//for testing
// go test ./internal/core/logger/