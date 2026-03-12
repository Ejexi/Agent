package taskengine

import (
	"context"
	"fmt"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
)

// Dispatcher coordinates the execution of an OSTask.
// It acts as the Application Service pipeline:
// Translator -> Security Gate -> Audit -> Executor
type Dispatcher struct {
	translator ports.OSTranslatorPort
	security   ports.SecurityGatePort
	executor   ports.CommandExecutorPort
	logger     shared_ports.Logger
}

// NewDispatcher creates a new task dispatcher pipeline.
func NewDispatcher(
	t ports.OSTranslatorPort, 
	s ports.SecurityGatePort, 
	e ports.CommandExecutorPort,
	l shared_ports.Logger,
) *Dispatcher {
	return &Dispatcher{
		translator: t,
		security:   s,
		executor:   e,
		logger:     l,
	}
}

// Submit executes the OSTask through the established pipeline.
// It guarantees that no command reaches the Executor without passing
// the Translator and the Security Gate.
func (d *Dispatcher) Submit(ctx context.Context, task domain.OSTask) domain.OSTaskResult {
	start := time.Now()
	
	result := domain.OSTaskResult{
		Status: domain.StatusRunning,
	}

	if d.logger != nil {
		d.logger.Debug(ctx, "TaskPipeline: Step 1 - Translating", shared_ports.Field{Key: "cmd", Value: task.OriginalCmd})
	}

	// 1. Translation
	// Convert standard LLM commands ("ls", "cat") to OS-specific commands.
	if d.translator != nil {
		task.OriginalCmd, task.Args = d.translator.Translate(task.OriginalCmd, task.Args)
	}

	if d.logger != nil {
		d.logger.Debug(ctx, "TaskPipeline: Step 2 - Evaluating Security", shared_ports.Field{Key: "translated_cmd", Value: task.OriginalCmd})
	}

	// 2. Security Gate (Sanitization & Warden Policy Check)
	if d.security != nil {
		if err := d.security.Evaluate(ctx, task); err != nil {
			// Failed security check
			result.Status = domain.StatusFailed
			result.Error = fmt.Errorf("security policy violation: %w", err)
			result.ExitCode = -1
			result.DurationMs = time.Since(start).Milliseconds()
			
			if d.logger != nil {
				d.logger.ErrorErr(ctx, result.Error, "TaskPipeline: Security Blocked Command")
			}
			return result
		}
	}

	if d.logger != nil {
		d.logger.Debug(ctx, "TaskPipeline: Step 3 - Executing", shared_ports.Field{Key: "cmd", Value: task.OriginalCmd})
	}

	// 3. Execution Phase
	if d.executor == nil {
		result.Status = domain.StatusFailed
		result.Error = fmt.Errorf("no command executor configured")
		result.ExitCode = -1
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Run the executable command
	execResult, err := d.executor.Execute(ctx, task)
	
	// Merge the execution result
	result.Stdout = execResult.Stdout
	result.Stderr = execResult.Stderr
	result.ExitCode = execResult.ExitCode
	result.Data = execResult.Data
	result.Error = err

	// Determine final task status
	if err != nil {
		result.Status = domain.StatusFailed
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = domain.StatusTimedOut
		} else if ctx.Err() == context.Canceled {
			result.Status = domain.StatusCancelled
		}
	} else {
		result.Status = domain.StatusCompleted
	}

	result.DurationMs = time.Since(start).Milliseconds()

	if d.logger != nil {
		d.logger.Debug(ctx, "TaskPipeline: Finished", 
			shared_ports.Field{Key: "status", Value: result.Status},
			shared_ports.Field{Key: "duration_ms", Value: result.DurationMs},
		)
	}

	return result
}
