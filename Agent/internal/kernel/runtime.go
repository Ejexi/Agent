package kernel

import (
	types "agent/internal/Types"
	"agent/internal/domain"
	"context"
)

// Runtime handles the execution of tools.
type Runtime struct {
	registry *Registry
}

// NewRuntime creates a new runtime.
func NewRuntime(registry *Registry) *Runtime {
	return &Runtime{
		registry: registry,
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

	// Runtime executes the tool (only the runtime should execute this)
	result, err := tool.Run(ctx, task)
	if err != nil {
		appErr := types.Wrapf(err, types.ErrCodeToolExecution, "failed to execute tool %s", task.Tool)
		return result, appErr
	}

	return result, nil
}
