package kernel

import (
	"fmt"
	"sync"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/orchestration"
	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/ports"
)

// Orchestrator executes a DAG-based execution plan with rollback support.
type Orchestrator struct {
	runtime  *Runtime
	auditLog ports.AuditLogPort
}

// NewOrchestrator creates a new DAG orchestrator.
func NewOrchestrator(runtime *Runtime, auditLog ports.AuditLogPort) *Orchestrator {
	return &Orchestrator{
		runtime:  runtime,
		auditLog: auditLog,
	}
}

// ExecutePlan runs the entire execution plan in topological order with rollback on failure.
func (o *Orchestrator) ExecutePlan(ctx *ExecutionContext, plan *orchestration.ExecutionPlan) error {
	levels, err := plan.TopologicalSort()
	if err != nil {
		return fmt.Errorf("orchestrator: failed to sort plan: %w", err)
	}

	// Audit: Plan execution started
	if o.auditLog != nil {
		taskIDs := make([]string, len(plan.Tasks))
		for i, t := range plan.Tasks {
			taskIDs[i] = t.ID
		}
		_ = o.auditLog.Record(ctx, security.AuditEntry{
			Action: security.AuditToolExecute,
			Actor:  ctx.PrincipalID,
			Target: plan.ID,
			Details: map[string]interface{}{
				"plan_name": plan.Name,
				"task_ids":  taskIDs,
			},
		})
	}

	// Execute level by level (tasks within a level can run in parallel)
	for _, level := range levels {
		if err := o.executeLevel(ctx, plan, level); err != nil {
			// Trigger rollback for all previously completed tasks
			o.rollback(ctx, plan)
			return fmt.Errorf("orchestrator: execution failed, rollback triggered: %w", err)
		}
	}

	return nil
}

// executeLevel runs all tasks in a single level in parallel.
// Uses a mutex to protect concurrent writes to plan.Tasks.
func (o *Orchestrator) executeLevel(ctx *ExecutionContext, plan *orchestration.ExecutionPlan, level []orchestration.DAGTask) error {
	var wg sync.WaitGroup
	var mu sync.Mutex // guards concurrent writes to plan.Tasks
	errCh := make(chan error, len(level))

	for i := range level {
		wg.Add(1)
		go func(dagTask orchestration.DAGTask) {
			defer wg.Done()

			mu.Lock()
			o.updateTaskStatus(plan, dagTask.ID, orchestration.DAGStatusRunning)
			mu.Unlock()

			result, err := o.runtime.Execute(ctx, dagTask.Task)
			if err != nil {
				mu.Lock()
				o.updateTaskStatus(plan, dagTask.ID, orchestration.DAGStatusFailed)
				mu.Unlock()
				errCh <- fmt.Errorf("task %s failed: %w", dagTask.ID, err)
				return
			}

			if !result.Success {
				mu.Lock()
				o.updateTaskStatus(plan, dagTask.ID, orchestration.DAGStatusFailed)
				mu.Unlock()
				errCh <- fmt.Errorf("task %s returned failure: %s", dagTask.ID, result.Error)
				return
			}

			mu.Lock()
			o.updateTaskStatus(plan, dagTask.ID, orchestration.DAGStatusSuccess)
			o.updateTaskResult(plan, dagTask.ID, &result)
			mu.Unlock()
		}(level[i])
	}

	wg.Wait()
	close(errCh)

	// Return the first error if any
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// rollback executes rollback tasks for completed tasks in reverse order.
// Logs errors but does not abort — all rollbacks must be attempted.
func (o *Orchestrator) rollback(ctx *ExecutionContext, plan *orchestration.ExecutionPlan) {
	rollbackChain := plan.GetRollbackChain()
	for _, rollbackTask := range rollbackChain {
		if _, err := o.runtime.Execute(ctx, rollbackTask); err != nil {
			// Log rollback failure but continue — partial rollback is better than none
			if o.auditLog != nil {
				_ = o.auditLog.Record(ctx, security.AuditEntry{
					Action: security.AuditToolResult,
					Actor:  "orchestrator",
					Target: rollbackTask.ID,
					Details: map[string]interface{}{
						"rollback_error": err.Error(),
					},
				})
			}
		}
	}
}

// updateTaskStatus updates the status of a specific task in the plan.
// Caller must hold the level mutex.
func (o *Orchestrator) updateTaskStatus(plan *orchestration.ExecutionPlan, taskID string, status orchestration.DAGTaskStatus) {
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == taskID {
			plan.Tasks[i].Status = status
			return
		}
	}
}

// updateTaskResult stores the result on a specific task in the plan.
// Caller must hold the level mutex.
func (o *Orchestrator) updateTaskResult(plan *orchestration.ExecutionPlan, taskID string, result *domain.Result) {
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == taskID {
			plan.Tasks[i].Result = result
			return
		}
	}
}
