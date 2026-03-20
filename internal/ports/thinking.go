package ports

import (
	"context"

	"github.com/SecDuckOps/shared/llm/domain"
)

// ThinkingPort allows the system to ask an AI for a rationale
// or risk assessment before executing a command.
type ThinkingPort interface {
	Analyze(ctx context.Context, command string, args []string) (domain.GenerationResult, error)
	Reflect(ctx context.Context, command string, args []string, stdout string, stderr string) (domain.GenerationResult, error)
	IsSafeToAutoExecute(ctx context.Context, command string, args []string) (bool, error)
}
