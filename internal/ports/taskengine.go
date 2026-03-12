package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
)

// OSTranslatorPort is responsible for translating a generic OS command
// into the platform-specific equivalent (e.g. converting "ls" to "cmd.exe /c dir" on Windows).
type OSTranslatorPort interface {
	Translate(cmd string, args []string) (string, []string)
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
}
