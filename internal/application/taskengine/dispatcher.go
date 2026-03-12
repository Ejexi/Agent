package taskengine

import (
	"context"
	"fmt"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
)

// Dispatcher coordinates the execution of an OSTask using a middleware pipeline.
type Dispatcher struct {
	pipeline TaskHandler
	logger   shared_ports.Logger
}

// NewDispatcher creates a new task dispatcher with a middleware-based pipeline.
func NewDispatcher(
	t ports.OSTranslatorPort,
	s ports.SecurityGatePort,
	e ports.CommandExecutorPort,
	l shared_ports.Logger,
) *Dispatcher {
	// 1. Base handler: The final step that actually executes the OS command.
	base := func(ctx context.Context, task *domain.OSTask) domain.OSTaskResult {
		if e == nil {
			return domain.OSTaskResult{
				Status: domain.StatusFailed,
				Error:  fmt.Errorf("no command executor configured"),
			}
		}
		execResult, err := e.Execute(ctx, *task)
		return domain.OSTaskResult{
			Stdout:   execResult.Stdout,
			Stderr:   execResult.Stderr,
			ExitCode: execResult.ExitCode,
			Data:     execResult.Data,
			Error:    err,
		}
	}

	// 2. Translation Middleware: Maps generic commands to OS-specific equivalents.
	translateMW := func(next TaskHandler) TaskHandler {
		return func(ctx context.Context, task *domain.OSTask) domain.OSTaskResult {
			if t != nil {
				task.OriginalCmd, task.Args = t.Translate(task.OriginalCmd, task.Args)
			}
			return next(ctx, task)
		}
	}

	// 3. Security Middleware: Enforces Warden policies and sanitization.
	securityMW := func(next TaskHandler) TaskHandler {
		return func(ctx context.Context, task *domain.OSTask) domain.OSTaskResult {
			if s != nil {
				if err := s.Evaluate(ctx, *task); err != nil {
					return domain.OSTaskResult{
						Status: domain.StatusFailed,
						Error:  err,
					}
				}
			}
			return next(ctx, task)
		}
	}

	// 4. Observability Middleware: Handles logging, timing, and error tracking.
	observabilityMW := func(next TaskHandler) TaskHandler {
		return func(ctx context.Context, task *domain.OSTask) domain.OSTaskResult {
			start := time.Now()
			if l != nil {
				l.Debug(ctx, "TaskPipeline: Execution started", shared_ports.Field{Key: "cmd", Value: task.OriginalCmd})
			}

			res := next(ctx, task)

			res.DurationMs = time.Since(start).Milliseconds()
			if l != nil {
				if res.Error != nil {
					l.ErrorErr(ctx, res.Error, "TaskPipeline: Execution failed")
				}
				l.Debug(ctx, "TaskPipeline: Execution finished",
					shared_ports.Field{Key: "status", Value: res.Status},
					shared_ports.Field{Key: "duration_ms", Value: res.DurationMs},
				)
			}
			return res
		}
	}

	// Order: Observability -> Translation -> Security -> Base (Execution)
	// (Note: Security comes after Translation to allow it to inspect the final OS-specific command)
	return &Dispatcher{
		pipeline: ChainMiddleware(base, observabilityMW, translateMW, securityMW),
		logger:   l,
	}
}

// Submit executes the OSTask through the established middleware pipeline.
func (d *Dispatcher) Submit(ctx context.Context, task domain.OSTask) domain.OSTaskResult {
	result := d.pipeline(ctx, &task)

	// Consolidate final status if not set by middlewares
	if result.Status == "" {
		if result.Error != nil {
			result.Status = domain.StatusFailed
		} else {
			result.Status = domain.StatusCompleted
		}
	}

	return result
}
