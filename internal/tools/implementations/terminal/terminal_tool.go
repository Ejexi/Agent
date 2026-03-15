package terminal

import (
	"context"
	"fmt"

	"github.com/SecDuckOps/agent/internal/application/taskengine"
	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/types"
)

// TerminalParams defines the inputs for the TerminalTool.
type TerminalParams struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Cwd       string            `json:"cwd,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	UsePTY    bool              `json:"use_pty,omitempty"`
	Cols      int               `json:"cols,omitempty"`
	Rows      int               `json:"rows,omitempty"`
	Streaming bool              `json:"streaming,omitempty"`
}

// TerminalTool acts as the bridge between the LLM Kernel Tool interface
// and the Hexagonal OS-Aware Task Middleware Pipeline.
type TerminalTool struct {
	base.BaseTypedTool[TerminalParams]
	dispatcher *taskengine.Dispatcher
}

// NewTerminalTool creates a new unified OS Execution tool.
func NewTerminalTool(dispatcher *taskengine.Dispatcher) *TerminalTool {
	t := &TerminalTool{
		dispatcher: dispatcher,
	}
	t.Impl = t
	return t
}

func (t *TerminalTool) Name() string { return "terminal" }

func (t *TerminalTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "terminal",
		Description: "Execute a command securely on the host operating system. Replaces both shell and filesystem tools.",
		Parameters: map[string]string{
			"command":   "string - The command name (e.g., ls, pwd, cat, git, go, etc.)",
			"args":      "[]string - Arguments to pass to the command",
			"cwd":       "string - Optional. Working directory (defaults to current dir)",
			"env":       "map[string]string - Optional. Environment variables. AVOID including standard system variables unless required for the specific command.",
			"use_pty":   "bool - Optional. Whether to use a PTY for interactive commands",
			"cols":      "int - Optional. Terminal columns for PTY",
			"rows":      "int - Optional. Terminal rows for PTY",
			"streaming": "bool - Optional. Whether to start as a streaming session",
		},
	}
}

func (t *TerminalTool) ParseParams(input map[string]interface{}) (TerminalParams, error) {
	params, err := base.DefaultParseParams[TerminalParams](input)
	if err != nil {
		return params, err
	}
	if params.Command == "" {
		return params, types.New(types.ErrCodeInvalidInput, "missing 'command' argument")
	}
	return params, nil
}

// Execute converts the Tool Request into an OSTask and pushes it through the Pipeline.
func (t *TerminalTool) Execute(ctx context.Context, params TerminalParams) (domain.Result, error) {
	// 1. Create the Core Domain OSTask
	task := domain.OSTask{
		OriginalCmd: params.Command,
		Args:        params.Args,
		Cwd:         params.Cwd,
		Env:         params.Env,
		UsePTY:      params.UsePTY,
		Cols:        params.Cols,
		Rows:        params.Rows,
	}

	if params.Streaming {
		sessionID, err := t.dispatcher.Start(ctx, task)
		if err != nil {
			return domain.Result{
				Success: false,
				Error:   fmt.Sprintf("failed to start streaming session: %v", err),
			}, nil
		}
		return domain.Result{
			Success: true,
			Status:  string(domain.StatusRunning),
			Data: map[string]interface{}{
				"session_id": sessionID,
				"status":     "streaming",
			},
		}, nil
	}

	// 2. Submit to the Dispatcher (The Application Service)
	taskResult := t.dispatcher.Submit(ctx, task)

	// 3. Map the Pipeline Output back to the LLM-compatible format
	result := domain.Result{
		Success: taskResult.Status == domain.StatusCompleted,
		Status:  string(taskResult.Status),
		Data: map[string]interface{}{
			"exit_code":   taskResult.ExitCode,
			"duration_ms": taskResult.DurationMs,
			"rationale":   taskResult.Rationale,
			"session_id":  taskResult.SessionID,
		},
	}

	// Prioritize AI reflection for the 'stdout' field to ensure it is displayed by default
	if taskResult.Reflection != "" {
		result.Data["stdout"] = taskResult.Reflection
		result.Data["raw_stdout"] = taskResult.Stdout
		result.Data["raw_stderr"] = taskResult.Stderr
	} else {
		if taskResult.Stdout != "" {
			result.Data["stdout"] = taskResult.Stdout
		}
		if taskResult.Stderr != "" {
			result.Data["stderr"] = taskResult.Stderr
		}
	}

	// Forward structural data if the executor/formatter added any
	for k, v := range taskResult.Data {
		result.Data[k] = v
	}

	if taskResult.Error != nil {
		result.Error = fmt.Sprintf("pipeline execution failed: %v", taskResult.Error)
	}

	return result, nil
}
