package kernel

import (
	"context"
	"testing"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/google/uuid"
)

// mockTool is a minimal tool for kernel tests.
type mockTool struct{}

func (m *mockTool) Name() string { return "mock" }
func (m *mockTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "mock",
		Description: "Test tool",
		Parameters:  map[string]string{"message": "string"},
	}
}
func (m *mockTool) ExecuteRaw(_ context.Context, input map[string]interface{}) (domain.Result, error) {
	msg, _ := input["message"].(string)
	return domain.Result{
		Success: true,
		Status:  "ok",
		Data:    map[string]interface{}{"message": msg},
	}, nil
}

func TestKernel_Execute(t *testing.T) {
	deps := Dependencies{}
	k := New(deps)

	// Register mock tool
	k.RegisterTool(&mockTool{})

	taskID := uuid.New().String()
	task := domain.Task{
		ID:   taskID,
		Tool: "mock",
		Args: map[string]any{
			"message": "hello duckops",
		},
	}

	result, err := k.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Kernel.Execute failed: %v", err)
	}

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

