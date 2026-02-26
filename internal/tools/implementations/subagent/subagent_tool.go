package subagent

import (
	"context"
	"fmt"

	"github.com/SecDuckOps/agent/internal/domain"
	sa "github.com/SecDuckOps/agent/internal/domain/subagent"
	tracker "github.com/SecDuckOps/agent/internal/adapters/subagent"
	"github.com/SecDuckOps/agent/internal/tools/base"
)

// SubagentParams — AOrchestra 4-tuple: (Instruction, Context, Tools, Model)
type SubagentParams struct {
	Description  string   `json:"description"`
	Instructions string   `json:"instructions"`
	Context      string   `json:"context,omitempty"`
	Tools        []string `json:"tools"`
	Model        string   `json:"model,omitempty"`
	MaxSteps     int      `json:"max_steps,omitempty"`
	Sandbox      bool     `json:"enable_sandbox,omitempty"`
	MaxRetries   int      `json:"max_retries,omitempty"`
	Provider     string   `json:"provider,omitempty"`
}

// SubagentTool is the MCP tool that the LLM calls to spawn a subagent.
type SubagentTool struct {
	base.BaseTypedTool[SubagentParams]
	tracker *tracker.Tracker
}

func NewSubagentTool(t *tracker.Tracker) *SubagentTool {
	tool := &SubagentTool{tracker: t}
	tool.Impl = tool
	return tool
}

func (t *SubagentTool) Name() string { return "dynamic_subagent_task" }

func (t *SubagentTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "dynamic_subagent_task",
		Description: `Create a dynamic subagent with full control over its configuration.
Implements the AOrchestra 4-tuple model (Instruction, Context, Tools, Model).

WHEN TO USE:
- When you need to delegate a specialized task to run asynchronously
- When passing context from previous attempts would help
- When the task requires specific tools with least-privilege access

CONTEXT GUIDELINES:
Include: relevant findings, key references (file paths, IDs), failed approaches to avoid.
Exclude: full conversation history, raw tool outputs, irrelevant tangents.

SANDBOX MODE (enable_sandbox=true):
- Runs subagent in isolated environment
- Subagent runs AUTONOMOUSLY without pausing for tool approval
- Non-sandbox subagents pause on each tool call for master agent approval

Use resume_subagent_task to approve/reject paused tool calls.`,
		Parameters: map[string]string{
			"description":    "string (required) - Short 3-5 word task description",
			"instructions":   "string (required) - What the subagent should do, with success criteria",
			"context":        "string (optional) - Curated context from previous work",
			"tools":          "[]string (required) - Tool names to grant (least-privilege)",
			"model":          "string (optional) - Model override (auto-downgraded for cost)",
			"max_steps":      "int (optional) - Max steps, default 30",
			"enable_sandbox": "bool (optional) - Run in isolated sandbox",
			"max_retries":    "int (optional) - Max retry attempts on failure (default: 3)",
			"provider":       "string (optional) - LLM provider override",
		},
	}
}

func (t *SubagentTool) ParseParams(input map[string]interface{}) (SubagentParams, error) {
	return base.DefaultParseParams[SubagentParams](input)
}

func (t *SubagentTool) Execute(ctx context.Context, params SubagentParams) (domain.Result, error) {
	if params.Instructions == "" {
		return domain.Result{Success: false, Error: "instructions is required"}, nil
	}
	if len(params.Tools) == 0 {
		return domain.Result{Success: false, Error: "tools array cannot be empty"}, nil
	}

	config := sa.SessionConfig{
		Description:  params.Description,
		Instructions: params.Instructions,
		Context:      params.Context,
		AllowedTools: params.Tools,
		Model:        params.Model,
		MaxSteps:     params.MaxSteps,
		Sandbox:      params.Sandbox,
		Provider:     params.Provider,
	}

	if params.MaxRetries > 0 {
		config.Retry = sa.RetryPolicy{MaxRetries: params.MaxRetries, DelayMs: 1000}
	}
	config.ApplyDefaults()

	sessionID, err := t.tracker.SpawnSubagent("", config)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   fmt.Sprintf("failed to spawn subagent: %v", err),
		}, nil
	}

	desc := params.Description
	if desc == "" {
		desc = params.Instructions[:min(50, len(params.Instructions))]
	}

	sandboxLabel := ""
	if params.Sandbox {
		sandboxLabel = " [sandboxed]"
	}

	return domain.Result{
		Success: true,
		Status:  "subagent_spawned",
		Data: map[string]interface{}{
			"session_id":  sessionID,
			"description": fmt.Sprintf("%s%s", desc, sandboxLabel),
			"tools":       params.Tools,
			"max_steps":   config.MaxSteps,
			"status":      string(sa.StatusPending),
		},
	}, nil
}
