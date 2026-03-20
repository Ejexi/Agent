package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
)

// HookRunnerPort executes user-defined hook scripts at lifecycle boundaries.
//
// All Run* methods follow the same contract:
//   - A nil return means "no hooks configured" — caller proceeds normally.
//   - A non-nil *HookOutput with Decision == HookDeny means blocked — caller
//     must abort and surface output.Reason to the user.
//   - A non-nil *HookOutput with Decision == HookAllow (or empty) means proceed.
//   - AfterTool and AfterScan are advisory: a deny decision is logged but
//     does not retroactively fail the completed operation.
type HookRunnerPort interface {
	// RunBeforeTool fires before a tool executes. Can block.
	RunBeforeTool(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error)

	// RunAfterTool fires after a tool completes. Advisory — cannot retroactively block.
	RunAfterTool(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error)

	// RunBeforeScan fires before MasterAgent starts a scan. Can block.
	RunBeforeScan(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error)

	// RunAfterScan fires after all scan subagents finish. Advisory.
	RunAfterScan(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error)

	// RunSessionStart fires when a new session begins. Advisory.
	RunSessionStart(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error)

	// RunSessionEnd fires when a session ends. Advisory.
	RunSessionEnd(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error)

	// List returns all registered hooks grouped by event type.
	List() map[domain.HookEventType][]domain.HookConfig
}
