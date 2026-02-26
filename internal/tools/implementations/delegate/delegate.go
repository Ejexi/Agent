package delegate

import (
	"context"
	"fmt"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	sa "github.com/SecDuckOps/agent/internal/domain/subagent"
	tracker "github.com/SecDuckOps/agent/internal/adapters/subagent"
	"github.com/SecDuckOps/agent/internal/tools/base"
)

type DelegateParams struct {
	CapabilityName string `json:"capability_name"`
	Objective      string `json:"objective"`
}

type TrackerClient interface {
	SpawnSubagent(parentID string, config sa.SessionConfig) (string, error)
}

type DelegateTool struct {
	base.BaseTypedTool[DelegateParams]
	tracker  TrackerClient
	registry *tracker.CapabilityRegistry
}

func NewDelegateTool(tracker TrackerClient, registry *tracker.CapabilityRegistry) *DelegateTool {
	tool := &DelegateTool{tracker: tracker, registry: registry}
	tool.Impl = tool
	return tool
}

func (t *DelegateTool) Name() string {
	return "delegate"
}

func (t *DelegateTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "delegate",
		Description: "Delegates a specific task to a specialized subagent based on a capability profile. You must provide the capability name and the task objective.",
		Parameters: map[string]string{
			"capability_name": "string (required) - The exact name of the specialized capability (e.g., 'scanner.filesystem')",
			"objective":       "string (required) - The specific task instructions for the subagent to perform",
		},
	}
}

func (t *DelegateTool) ParseParams(input map[string]interface{}) (DelegateParams, error) {
	return base.DefaultParseParams[DelegateParams](input)
}

func (t *DelegateTool) Execute(ctx context.Context, params DelegateParams) (domain.Result, error) {
	if params.CapabilityName == "" || params.Objective == "" {
		return domain.Result{Success: false, Error: "capability_name and objective are required"}, nil
	}

	cap, ok := t.registry.Get(params.CapabilityName)
	if !ok {
		available := t.registry.List()
		var names []string
		for _, c := range available {
			names = append(names, c.Name)
		}
		errMsg := fmt.Sprintf("Capability '%s' not found. Available capabilities: %s", params.CapabilityName, strings.Join(names, ", "))
		return domain.Result{Success: false, Error: errMsg}, nil
	}

	config := sa.SessionConfig{
		Description:  fmt.Sprintf("Delegated: %s", cap.Name),
		Instructions: fmt.Sprintf("SYSTEM ROLE: %s\n\nOBJECTIVE: %s", cap.SystemPrompt, params.Objective),
		AllowedTools: cap.AllowedTools,
		Sandbox:      false,
		MaxSteps:     30,
	}
	config.ApplyDefaults()

	sessionID, err := t.tracker.SpawnSubagent("", config)
	if err != nil {
		return domain.Result{Success: false, Error: fmt.Sprintf("failed to spawn delegated subagent: %v", err)}, nil
	}

	return domain.Result{
		Success: true,
		Status:  "subagent_spawned",
		Data: map[string]interface{}{
			"session_id":      sessionID,
			"capability_name": cap.Name,
			"description":     config.Description,
			"tools":           cap.AllowedTools,
			"status":          string(sa.StatusPending),
		},
	}, nil
}
