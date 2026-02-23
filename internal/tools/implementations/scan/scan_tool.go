package scan

import (
	"context"
	"fmt"

	"github.com/SecDuckOps/shared/llm/domain"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/tools/base"
)

// ScanTool implements the domain.Tool interface.
type ScanTool struct {
	// The tool requires these ports to function.
	// It doesn't care if they are implemented by a Mock, CLI, or an actual API.
	llmRegistry domain.LLMRegistry
	memory      ports.MemoryPort
}

// ExecuteRaw implements [base.Tool].
func (t *ScanTool) ExecuteRaw(ctx context.Context, input map[string]interface{}) (agent_domain.Result, error) {
	// Delegate to Run by creating a virtual task
	return t.Run(ctx, agent_domain.Task{
		ID:   "direct_exec_" + t.Name(),
		Tool: t.Name(),
		Args: input,
	})
}

// Schema implements [base.Tool].
func (t *ScanTool) Schema() base.ToolSchema {
	return base.ToolSchema{
		Name:        "scan",
		Description: "Perform a security scan on a target using LLM analysis and memory storage.",
		Parameters: map[string]string{
			"target":      "string - The target to scan",
			"ai_provider": "string - Optional. The AI provider to use (default: openai)",
		},
	}
}

// NewScanTool creates a new instance of ScanTool.
// Notice how we inject the exact Ports this tool needs.
func NewScanTool(llmRegistry domain.LLMRegistry, memory ports.MemoryPort) *ScanTool {
	return &ScanTool{
		llmRegistry: llmRegistry,
		memory:      memory,
	}
}

// Name returns the name of the tool.
func (t *ScanTool) Name() string {
	return "scan"
}

// Run executes the scanning process.
func (t *ScanTool) Run(ctx context.Context, task agent_domain.Task) (agent_domain.Result, error) {
	// Example flow:
	// 1- Parse the Target from task args
	target, ok := task.Args["target"].(string)
	if !ok {
		return agent_domain.Result{
			TaskID:  task.ID,
			Success: false,
			Status:  "failed",
			Error:   "missing target argument",
		}, nil
	}

	// 2- Save target to memory (Example usage of MemoryPort)
	if t.memory != nil {
		_ = t.memory.Save(ctx, fmt.Sprintf("scan_target_%s", task.ID), target)
	}

	// 3- Analyze with LLM (Example usage of LLMRegistry)
	llmReport := "Dummy LLM Report for " + target
	if t.llmRegistry != nil {
		providerName := "openai" // Assume openai is the default
		if p, ok := task.Args["ai_provider"].(string); ok {
			providerName = p
		}

		provider := t.llmRegistry.Get(providerName)
		if provider != nil {
			report, err := provider.Generate(ctx, []domain.Message{
				{Role: domain.RoleUser, Content: "Analyze this scan target: " + target},
			}, nil)
			if err == nil {
				llmReport = report
			}
		}
	}

	// 4- Return Result
	return agent_domain.Result{
		TaskID:  task.ID,
		Success: true,
		Status:  "scan completed successfully",
		Data: map[string]interface{}{
			"target":     target,
			"llm_report": llmReport,
		},
	}, nil
}
