package shell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/types"
)

const (
	// defaultTimeout for command execution.
	defaultTimeout = 30 * time.Second

	// maxOutputBytes caps captured stdout/stderr.
	maxOutputBytes = 256 * 1024 // 256 KB
)

// ShellParams defines the typed parameters for the shell tool.
type ShellParams struct {
	Command   string   `json:"command"`             // Canonical command name (ls, cat, grep, git, etc.)
	Args      []string `json:"args,omitempty"`       // Arguments to pass to the command
	Workspace string   `json:"workspace,omitempty"`  // Working directory (defaults to current)
	Timeout   int      `json:"timeout,omitempty"`    // Timeout in seconds (default: 30)
	UsePTY    bool     `json:"use_pty,omitempty"`    // Whether to use a PTY
}

// ShellTool executes safe OS commands via ShellExecutionPort with Warden policy gating.
type ShellTool struct {
	base.BaseTypedTool[ShellParams]
	warden ports.WardenPort
	shell  ports.ShellExecutionPort
}

// NewShellTool creates a new ShellTool with Warden policy enforcement.
func NewShellTool(warden ports.WardenPort, shell ports.ShellExecutionPort) *ShellTool {
	t := &ShellTool{
		warden: warden,
		shell:  shell,
	}
	t.Impl = t
	return t
}

func (t *ShellTool) Name() string { return "shell" }

func (t *ShellTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "shell",
		Description: "Execute safe OS commands (ls, cat, grep, git, find, etc.) with security policy enforcement. Commands are restricted to a safe allowlist and arguments are sanitized.",
		Parameters: map[string]string{
			"command":   "string - The command to execute (e.g., ls, cat, grep, git, find, tree, wc, head, tail, du, which, stat)",
			"args":      "[]string - Arguments and flags to pass to the command",
			"workspace": "string - Optional. Working directory for the command (must be within project boundaries)",
			"timeout":   "int - Optional. Timeout in seconds (default: 30, max: 120)",
			"use_pty":   "bool - Optional. Whether to use a PTY for interactive commands",
		},
	}
}

func (t *ShellTool) ParseParams(input map[string]interface{}) (ShellParams, error) {
	params, err := base.DefaultParseParams[ShellParams](input)
	if err != nil {
		return params, err
	}
	if params.Command == "" {
		return params, types.New(types.ErrCodeInvalidInput, "missing 'command' argument")
	}
	if params.Timeout <= 0 {
		params.Timeout = int(defaultTimeout.Seconds())
	}
	if params.Timeout > 120 {
		params.Timeout = 120
	}
	return params, nil
}

func (t *ShellTool) Execute(ctx context.Context, params ShellParams) (domain.Result, error) {
	// 1. Validate command against allowlist
	mapping, err := SanitizeCommand(params.Command, AllowedCommands)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   fmt.Sprintf("command validation failed: %v", err),
		}, nil
	}

	// 2. Sanitize arguments
	if err := SanitizeArgs(params.Args); err != nil {
		return domain.Result{
			Success: false,
			Error:   fmt.Sprintf("argument validation failed: %v", err),
		}, nil
	}

	// 3. Validate workspace path if provided
	workspace := params.Workspace
	if workspace == "" {
		workspace = "."
	}
	if err := ValidateWorkspace(workspace); err != nil {
		return domain.Result{
			Success: false,
			Error:   fmt.Sprintf("workspace validation failed: %v", err),
		}, nil
	}

	// 4. Build the command
	binary := mapping.Binary()
	args := params.Args

	// Windows special handling: wrap Unix commands in cmd.exe /C
	if runtime.GOOS == "windows" && binary == "cmd" {
		winArgs := WindowsShellArgs(params.Command, params.Args)
		if winArgs == nil {
			// Commands like head/tail that need PowerShell
			return t.executePowerShell(ctx, params)
		}
		args = winArgs
		binary = "cmd"
	}

	// 5. Execute via ShellExecutionPort
	timeout := time.Duration(params.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	osTask := domain.OSTask{
		OriginalCmd: binary,
		Args:        args,
		Cwd:         workspace,
		UsePTY:      params.UsePTY,
	}

	shellResult, err := t.shell.Execute(ctx, osTask)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   fmt.Sprintf("command execution failed: %v", err),
		}, nil
	}

	// Build result
	result := domain.Result{
		Success: shellResult.ExitCode == 0,
		Status:  "completed",
		Data: map[string]interface{}{
			"command":   params.Command,
			"args":      params.Args,
			"stdout":    shellResult.Stdout,
			"stderr":    shellResult.Stderr,
			"exit_code": shellResult.ExitCode,
		},
	}

	if shellResult.ExitCode != 0 {
		if shellResult.Error != nil {
			result.Error = shellResult.Error.Error()
		} else {
			result.Error = fmt.Sprintf("command exited with code %d", shellResult.ExitCode)
		}
	}

	return result, nil
}

// executePowerShell handles Windows commands that need PowerShell (head, tail).
func (t *ShellTool) executePowerShell(ctx context.Context, params ShellParams) (domain.Result, error) {
	var psCmd string
	switch params.Command {
	case "head":
		if len(params.Args) > 0 {
			psCmd = fmt.Sprintf("Get-Content '%s' -Head 10", params.Args[len(params.Args)-1])
		}
	case "tail":
		if len(params.Args) > 0 {
			psCmd = fmt.Sprintf("Get-Content '%s' -Tail 10", params.Args[len(params.Args)-1])
		}
	}

	if psCmd == "" {
		return domain.Result{
			Success: false,
			Error:   "insufficient arguments for " + params.Command,
		}, nil
	}

	timeout := time.Duration(params.Timeout) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "powershell", "-NoProfile", "-Command", psCmd)
	cmd.Dir = params.Workspace

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, limit: maxOutputBytes}
	cmd.Stderr = &limitedWriter{w: &stderr, limit: maxOutputBytes}

	execErr := cmd.Run()

	result := domain.Result{
		Success: execErr == nil,
		Status:  "completed",
		Data: map[string]interface{}{
			"command":   params.Command,
			"args":      params.Args,
			"stdout":    stdout.String(),
			"stderr":    stderr.String(),
			"exit_code": 0,
		},
	}

	if execErr != nil {
		result.Error = fmt.Sprintf("command failed: %v", execErr)
		result.Data["exit_code"] = -1
	}

	return result, nil
}

// limitedWriter caps the amount of data written.
type limitedWriter struct {
	w       *bytes.Buffer
	limit   int
	written int
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	remaining := lw.limit - lw.written
	if remaining <= 0 {
		return len(p), nil // silently discard
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	n, err := lw.w.Write(p)
	lw.written += n
	return n, err
}
