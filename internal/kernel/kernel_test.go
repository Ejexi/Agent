package kernel

import (
	"context"
	"testing"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/tools/echo"
	"github.com/google/uuid"
)

func TestKernel_Execute(t *testing.T) {
	// Initialize Kernel with empty dependencies
	deps := Dependencies{}
	k := New(deps)

	// Register Echo tool
	echoTool := echo.NewEchoTool()
	k.RegisterTool(echoTool)

	// Create a task
	taskID := uuid.New().String()
	task := domain.Task{
		ID:   taskID,
		Tool: "echo",
		Args: map[string]any{
			"message": "hello duckops",
		},
	}

	// Execute task
	result, err := k.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Kernel.Execute failed: %v", err)
	}

	// Verify result
	if !result.Success {
		t.Errorf("expected success, got failure: %v", result.Error)
	}

	if result.TaskID != taskID {
		t.Errorf("expected TaskID %v, got %v", taskID, result.TaskID)
	}

	if result.Data["message"] != "hello duckops" {
		t.Errorf("expected message 'hello duckops', got %v", result.Data["message"])
	}
}
