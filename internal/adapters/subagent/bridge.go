package subagent

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
)

// KernelBridge wraps the Kernel to satisfy both ToolExecutor and LLMProvider
// without the subagent package importing the kernel package directly.
type KernelBridge struct {
	ExecuteFn    func(ctx context.Context, task domain.Task) (domain.Result, error)
	GetSchemasFn func(allowedTools []string) []domain.ToolSchema
	LLMRegistry  shared_domain.LLMRegistry
}

// Execute delegates to the kernel's Execute method.
func (b *KernelBridge) Execute(ctx context.Context, task domain.Task) (domain.Result, error) {
	return b.ExecuteFn(ctx, task)
}

// GetToolSchemas delegates to the kernel's GetToolSchemas method.
func (b *KernelBridge) GetToolSchemas(allowedTools []string) []domain.ToolSchema {
	return b.GetSchemasFn(allowedTools)
}

// Get delegates to the LLM registry.
func (b *KernelBridge) Get(name string) shared_domain.LLM {
	return b.LLMRegistry.Get(name)
}

// List delegates to the LLM registry.
func (b *KernelBridge) List() []string {
	return b.LLMRegistry.List()
}
