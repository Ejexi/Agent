package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
)

// OSTranslatorPort is responsible for translating a generic OS command
// into the platform-specific equivalent (e.g. converting "ls" to "cmd.exe /c dir" on Windows).
type OSTranslatorPort interface {
	Translate(task *domain.OSTask)
	Normalize(cmd string, args []string) (string, []string)
}

// SecurityGatePort is responsible for validating the safety of an OSTask.
// This typically includes arguments sanitization, command allowlisting,
// and Authorization via Warden Cedar policies.
type SecurityGatePort interface {
	Evaluate(ctx context.Context, task domain.OSTask) error
}

// CommandExecutorPort is the final step in the pipeline.
// It executes the command on the target environment (e.g., local Host OS or an isolated container sandbox).
type CommandExecutorPort interface {
	Execute(ctx context.Context, task domain.OSTask) (domain.OSTaskResult, error)
	Start(ctx context.Context, task domain.OSTask) (string, error)
	Kill(ctx context.Context, sessionID string) error
	Resize(ctx context.Context, sessionID string, cols, rows int) error
	Subscribe(ctx context.Context, sessionID string) (<-chan domain.ShellOutput, error)
	GetSession(ctx context.Context, sessionID string) (domain.ShellSession, error)
	ListSessions(ctx context.Context) ([]domain.ShellSession, error)
}

// ShellExecutionPort handles the creation and management of shell processes.
type ShellExecutionPort = CommandExecutorPort

// ShellLifecyclePort tracks active shell sessions and provides updates.
type ShellLifecyclePort interface {
	Subscribe(ctx context.Context, sessionID string) (<-chan domain.ShellOutput, error)
	GetSession(ctx context.Context, sessionID string) (domain.ShellSession, error)
	ListSessions(ctx context.Context) ([]domain.ShellSession, error)
}
