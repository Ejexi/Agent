package scan

import (
	"agent/internal/domain"
	"agent/internal/ports"
	"context"
	"fmt"
)

// ScanTool implements the domain.Tool interface.
type ScanTool struct {
	// The tool requires these ports to function.
	// It doesn't care if they are implemented by a Mock, CLI, or an actual API.
	llmRegistry ports.LLMRegistry
	memory      ports.MemoryPort
}

// NewScanTool creates a new instance of ScanTool.
// Notice how we inject the exact Ports this tool needs.
func NewScanTool(llmRegistry ports.LLMRegistry, memory ports.MemoryPort) *ScanTool {
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
func (t *ScanTool) Run(ctx context.Context, task domain.Task) (domain.Result, error) {
	// Example flow:
	// 1- Parse the Target from task args
	target, ok := task.Args["target"].(string)
	if !ok {
		return domain.Result{
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
			report, err := provider.Generate(ctx, "Analyze this scan target: "+target)
			if err == nil {
				llmReport = report
			}
		}
	}

	// 4- Return Result
	return domain.Result{
		TaskID:  task.ID,
		Success: true,
		Status:  "scan completed successfully",
		Data: map[string]interface{}{
			"target":     target,
			"llm_report": llmReport,
		},
	}, nil
}
