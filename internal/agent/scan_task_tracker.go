package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/SecDuckOps/shared/scanner/domain"
	"github.com/SecDuckOps/shared/types"
)

// ScanTaskTracker tracks all scan tasks spawned by MasterAgent.
// Thread-safe. Separate from the LLM subagent Tracker.
type ScanTaskTracker struct {
	mu    sync.RWMutex
	tasks map[TaskID]*ScanTask
}

// NewScanTaskTracker creates an empty tracker.
func NewScanTaskTracker() *ScanTaskTracker {
	return &ScanTaskTracker{
		tasks: make(map[TaskID]*ScanTask),
	}
}

// Register adds a new task and returns its ID.
func (t *ScanTaskTracker) Register(category ScanCategory, targetPath string) *ScanTask {
	task := &ScanTask{
		ID:         generateTaskID(),
		Category:   category,
		TargetPath: targetPath,
		Status:     ScanTaskStarting,
	}
	t.mu.Lock()
	t.tasks[task.ID] = task
	t.mu.Unlock()
	return task
}

// SetRunning transitions a task to running.
func (t *ScanTaskTracker) SetRunning(id TaskID) {
	t.mu.Lock()
	if task, ok := t.tasks[id]; ok {
		task.Status = ScanTaskRunning
	}
	t.mu.Unlock()
}

// SetCompleted marks a task done with its findings.
func (t *ScanTaskTracker) SetCompleted(id TaskID, findings []domain.Finding) {
	t.mu.Lock()
	if task, ok := t.tasks[id]; ok {
		task.Status = ScanTaskCompleted
		task.Findings = findings
	}
	t.mu.Unlock()
}

// SetFailed marks a task as failed with an error message.
func (t *ScanTaskTracker) SetFailed(id TaskID, errMsg string) {
	t.mu.Lock()
	if task, ok := t.tasks[id]; ok {
		task.Status = ScanTaskFailed
		task.Error = errMsg
	}
	t.mu.Unlock()
}

// Get returns a task by ID.
func (t *ScanTaskTracker) Get(id TaskID) (*ScanTask, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	task, ok := t.tasks[id]
	return task, ok
}

// ListAll returns a snapshot of all tasks.
func (t *ScanTaskTracker) ListAll() []*ScanTask {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]*ScanTask, 0, len(t.tasks))
	for _, task := range t.tasks {
		result = append(result, task)
	}
	return result
}

// WaitForAll blocks until all taskIDs reach a terminal state or ctx/timeout fires.
// Uses a ticker (500ms) to avoid spinning, same pattern as Phase 2 spec.
func (t *ScanTaskTracker) WaitForAll(ctx context.Context, ids []TaskID, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				// Mark still-running tasks as timed out
				t.mu.Lock()
				for _, id := range ids {
					if task, ok := t.tasks[id]; ok {
						switch task.Status {
						case ScanTaskStarting, ScanTaskRunning:
							task.Status = ScanTaskTimedOut
							task.Error = fmt.Sprintf("timed out after %s", timeout)
						}
					}
				}
				t.mu.Unlock()
				return types.Newf(types.ErrCodeExecutionFailed, "scan timed out after %s", timeout)
			}
			if t.allTerminal(ids) {
				return nil
			}
		}
	}
}

func (t *ScanTaskTracker) allTerminal(ids []TaskID) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, id := range ids {
		task, ok := t.tasks[id]
		if !ok {
			continue
		}
		switch task.Status {
		case ScanTaskCompleted, ScanTaskFailed, ScanTaskCancelled, ScanTaskTimedOut:
			// terminal
		default:
			return false
		}
	}
	return true
}

// generateTaskID creates a short 6-char hex ID.
func generateTaskID() TaskID {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return TaskID(hex.EncodeToString(b))
}
