package delegate

import (
	"context"
	"testing"

	"github.com/SecDuckOps/agent/internal/domain/subagent"
	"github.com/SecDuckOps/agent/internal/ports"
	tracker "github.com/SecDuckOps/agent/internal/adapters/subagent"
)

// mockTracker is a minimal mock for the tracker interface
type mockTracker struct {
	spawnedSessionID string
}

func (m *mockTracker) SpawnSubagent(parentID string, config subagent.SessionConfig) (string, error) {
	m.spawnedSessionID = "sub_mocked123"
	return m.spawnedSessionID, nil
}

func (m *mockTracker) Resume(ctx context.Context, sessionID string) (string, error) {
	return "Mocked subagent final result", nil
}

func TestDelegateTool_Execute(t *testing.T) {
	// 1. Setup Registry with a test capability
	registry := tracker.NewCapabilityRegistry()
	registry.Sync([]ports.Capability{
		{
			Name:         "test.capability",
			Description:  "A test capability",
			SystemPrompt: "You are a test subagent.",
			AllowedTools: []string{"echo"},
		},
	})

	// 2. Setup Mock Tracker
	mockT := &mockTracker{}

	// 3. Initialize Delegate Tool (TrackerClient is first argument, registry is second)
	tool := NewDelegateTool(mockT, registry)

	// 4. Test missing capability
	paramsMissing := DelegateParams{
		CapabilityName: "nonexistent.capability",
		Objective:      "Do nothing",
	}
	resMissing, err := tool.Execute(context.Background(), paramsMissing)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resMissing.Success {
		t.Fatalf("expected failure for nonexistent capability, got success")
	}

	// 5. Test successful delegation
	paramsSuccess := DelegateParams{
		CapabilityName: "test.capability",
		Objective:      "Run a test",
	}
	resSuccess, err := tool.Execute(context.Background(), paramsSuccess)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !resSuccess.Success {
		t.Fatalf("expected success for valid capability, got failure: %v", resSuccess.Error)
	}

	// 6. Verify spawned ID
	if spawnedID, ok := resSuccess.Data["session_id"].(string); !ok || spawnedID != "sub_mocked123" {
		t.Errorf("expected session_id 'sub_mocked123', got %v", resSuccess.Data["session_id"])
	}
}
