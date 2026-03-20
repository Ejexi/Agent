package taskengine

import (
	"context"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/shared/types"
)

// HookMiddleware wraps every tool execution with BeforeTool and AfterTool hooks.
//
// Position in the pipeline (outermost — fires first, observes last):
//
//	HookMiddleware → ReflectionMW → ThinkingMW → ObservabilityMW → TranslateMW → SecurityMW → base
//
// BeforeTool can block execution. AfterTool is advisory.
// If runner is nil the middleware is a transparent no-op.
func HookMiddleware(runner ports.HookRunnerPort) TaskMiddleware {
	return func(next TaskHandler) TaskHandler {
		return func(ctx context.Context, task *domain.OSTask) domain.OSTaskResult {
			if runner == nil {
				return next(ctx, task)
			}

			// ── BeforeTool ────────────────────────────────────────────────────
			input := domain.HookInput{
				Event:     domain.HookBeforeTool,
				ToolName:  task.OriginalCmd,
				ToolArgs:  taskArgsMap(task),
				SessionID: task.SessionID,
				Timestamp: time.Now().UTC(),
			}

			output, err := runner.RunBeforeTool(ctx, input)
			if err != nil {
				reason := "BeforeTool hook blocked execution"
				if output != nil && output.Reason != "" {
					reason = output.Reason
				}
				return domain.OSTaskResult{
					Status: domain.StatusFailed,
					Error:  types.Newf(types.ErrCodeSecurityViolation, "BeforeTool hook: %s", reason),
				}
			}

			// ── Execute ───────────────────────────────────────────────────────
			start := time.Now()
			result := next(ctx, task)

			// ── AfterTool ─────────────────────────────────────────────────────
			afterInput := domain.HookInput{
				Event:    domain.HookAfterTool,
				ToolName: task.OriginalCmd,
				ToolArgs: taskArgsMap(task),
				Result: &domain.HookToolResult{
					Stdout:     result.Stdout,
					Stderr:     result.Stderr,
					ExitCode:   result.ExitCode,
					DurationMs: time.Since(start).Milliseconds(),
					Status:     string(result.Status),
					Error:      errorStr(result.Error),
				},
				SessionID: task.SessionID,
				Timestamp: time.Now().UTC(),
			}

			// AfterTool is advisory — ignore errors
			runner.RunAfterTool(ctx, afterInput) //nolint:errcheck

			return result
		}
	}
}

// taskArgsMap converts an OSTask to a key/value map for hook input.
func taskArgsMap(task *domain.OSTask) map[string]interface{} {
	m := map[string]interface{}{
		"command": task.OriginalCmd,
	}
	if len(task.Args) > 0 {
		m["args"] = task.Args
	}
	if task.Cwd != "" {
		m["cwd"] = task.Cwd
	}
	return m
}

func errorStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
