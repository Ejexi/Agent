package kernel

import (
	"context"
	"sync"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/ports"
	types "github.com/SecDuckOps/shared/types"
)

// Runtime handles the execution of tools.
type Runtime struct {
	registry *Registry
	auditLog ports.AuditLogPort
}

// NewRuntime creates a new runtime.
func NewRuntime(registry *Registry, auditLog ports.AuditLogPort) *Runtime {
	return &Runtime{
		registry: registry,
		auditLog: auditLog,
	}
}

// Execute runs a tool based on the provided task.
func (r *Runtime) Execute(ctx context.Context, task domain.Task) (domain.Result, error) {
	if r.registry == nil {
		err := types.New(types.ErrCodeInternal, "runtime registry is not initialized")
		return domain.Result{
			TaskID:  task.ID,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	tool, exists := r.registry.Get(task.Tool)
	if !exists {
		err := types.Newf(types.ErrCodeToolNotFound, "tool not found: %s", task.Tool)
		return domain.Result{
			TaskID:  task.ID,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// 1. Audit Log: Tool Execution Started
	if r.auditLog != nil {
		_ = r.auditLog.Record(ctx, security.AuditEntry{
			SessionID: task.SessionID, // Requires SessionID on Task
			Action:    security.AuditToolExecute,
			Actor:     "kernel",
			Target:    task.Tool,
			Details: map[string]interface{}{
				"args": task.Args,
			},
			Timestamp: time.Now(),
		})
	}

	// Runtime executes the tool (only the runtime should execute this)
	result, err := tool.ExecuteRaw(ctx, task.Args)
	if err != nil {
		appErr := types.Wrapf(err, types.ErrCodeToolExecution, "failed to execute tool %s", task.Tool)
		return result, appErr
	}

	// Ensure the result has the correct TaskID
	result.TaskID = task.ID

	// 2. Audit Log: Tool Execution Completed
	if r.auditLog != nil {
		_ = r.auditLog.Record(ctx, security.AuditEntry{
			SessionID: task.SessionID,
			Action:    security.AuditToolResult,
			Actor:     "kernel",
			Target:    task.Tool,
			Details: map[string]interface{}{
				"success": result.Success,
				"error":   result.Error,
			},
			Timestamp: time.Now(),
		})
	}

	return result, nil
}

// ExecuteBatch runs multiple tools in parallel.
func (r *Runtime) ExecuteBatch(ctx context.Context, tasks []domain.Task) ([]domain.Result, error) {
	if r.registry == nil {
		return nil, types.New(types.ErrCodeInternal, "runtime registry is not initialized")
	}

	results := make([]domain.Result, len(tasks))
	errors := make([]error, len(tasks))
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t domain.Task) {
			defer wg.Done()
			res, err := r.Execute(ctx, t)
			results[idx] = res
			errors[idx] = err
		}(i, task)
	}

	wg.Wait()

	// Return the first error encountered, if any (or we could wrap all of them)
	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}
