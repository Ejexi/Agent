package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
)

// ToolExecutor is the interface for executing tools.
// The Kernel satisfies this — consumers never import the kernel directly.
type ToolExecutor interface {
	Execute(ctx context.Context, task domain.Task) (domain.Result, error)
}

// ToolSchemaProvider provides tool schemas for the LLM system prompt.
type ToolSchemaProvider interface {
	GetToolSchemas(allowedTools []string) []domain.ToolSchema
}
