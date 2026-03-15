package orchestration

import (
	"github.com/SecDuckOps/agent/internal/domain"
)

// DAGTaskStatus tracks the execution state of a single DAG node.
type DAGTaskStatus string

const (
	DAGStatusPending  DAGTaskStatus = "pending"
	DAGStatusRunning  DAGTaskStatus = "running"
	DAGStatusSuccess  DAGTaskStatus = "success"
	DAGStatusFailed   DAGTaskStatus = "failed"
	DAGStatusRolledBack DAGTaskStatus = "rolled_back"
	DAGStatusSkipped  DAGTaskStatus = "skipped"
)

// DAGTask represents a single node in a task execution graph.
type DAGTask struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Dependencies []string      `json:"dependencies,omitempty"` // IDs of tasks that must complete before this one
	Task         domain.Task   `json:"task"`
	RollbackTask *domain.Task  `json:"rollback_task,omitempty"` // Executed on downstream failure
	Status       DAGTaskStatus `json:"status"`
	Result       *domain.Result `json:"result,omitempty"`
}

// ExecutionPlan represents a complete DAG of tasks to execute.
type ExecutionPlan struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tasks       []DAGTask `json:"tasks"`
}

// TopologicalSort returns the tasks in execution order based on dependencies.
// Returns an error if a cycle is detected.
func (p *ExecutionPlan) TopologicalSort() ([][]DAGTask, error) {
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)
	taskMap := make(map[string]DAGTask)

	for _, t := range p.Tasks {
		taskMap[t.ID] = t
		if _, ok := inDegree[t.ID]; !ok {
			inDegree[t.ID] = 0
		}
		for _, dep := range t.Dependencies {
			dependents[dep] = append(dependents[dep], t.ID)
			inDegree[t.ID]++
		}
	}

	// Find initial ready tasks (no dependencies)
	var levels [][]DAGTask
	ready := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			ready = append(ready, id)
		}
	}

	processed := 0
	for len(ready) > 0 {
		level := make([]DAGTask, 0, len(ready))
		nextReady := make([]string, 0)

		for _, id := range ready {
			level = append(level, taskMap[id])
			processed++

			for _, dep := range dependents[id] {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					nextReady = append(nextReady, dep)
				}
			}
		}

		levels = append(levels, level)
		ready = nextReady
	}

	if processed != len(p.Tasks) {
		return nil, domain.ErrCyclicDependency
	}

	return levels, nil
}

// GetRollbackChain returns the rollback tasks for all completed tasks in reverse order.
func (p *ExecutionPlan) GetRollbackChain() []domain.Task {
	var rollbacks []domain.Task
	for i := len(p.Tasks) - 1; i >= 0; i-- {
		t := p.Tasks[i]
		if t.Status == DAGStatusSuccess && t.RollbackTask != nil {
			rollbacks = append(rollbacks, *t.RollbackTask)
		}
	}
	return rollbacks
}
