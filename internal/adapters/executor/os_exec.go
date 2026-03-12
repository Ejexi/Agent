package executor

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
)

// OSExecAdapter implements the CommandExecutorPort using Go's native os/exec.
// It executes commands on the Host OS securely as the final stage of the Pipeline.
type OSExecAdapter struct {
	logger shared_ports.Logger
}

// NewOSExecAdapter creates a new executor that runs tasks on the native host.
func NewOSExecAdapter(logger shared_ports.Logger) ports.CommandExecutorPort {
	return &OSExecAdapter{logger: logger}
}

// Execute performs the raw process fork/exec.
// Since this is the ExecAdapter, it assumes SecurityGate/Warden have ALREADY approved this task.
func (a *OSExecAdapter) Execute(ctx context.Context, task domain.OSTask) (domain.OSTaskResult, error) {
	cmd := exec.CommandContext(ctx, task.OriginalCmd, task.Args...)
	
	if task.Cwd != "" {
		cmd.Dir = task.Cwd
	}

	// Build environment variables if provided
	if len(task.Env) > 0 {
		env := cmd.Environ() // inherit from host
		for k, v := range task.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil // we return the exit code + stderr, not an immediate go error
		} else {
			// e.g. executable not found in PATH, context cancelled/timedout
			a.logger.ErrorErr(ctx, err, "Failed to start process via OSExecAdapter", 
				shared_ports.Field{Key: "cmd", Value: task.OriginalCmd})
			return domain.OSTaskResult{
				ExitCode:   -1,
				DurationMs: duration,
			}, err
		}
	}

	return domain.OSTaskResult{
		Status:     domain.StatusCompleted,
		Stdout:     strings.TrimSpace(stdout.String()),
		Stderr:     strings.TrimSpace(stderr.String()),
		ExitCode:   exitCode,
		DurationMs: duration,
	}, nil
}
