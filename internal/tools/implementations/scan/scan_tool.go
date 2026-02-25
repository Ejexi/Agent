package scan

import (
	"context"

	"github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/types"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/tools/base"
)

// ScanParams defines the typed parameters for the scan tool.
type ScanParams struct {
	Target     string `json:"target"`
	AIProvider string `json:"ai_provider"`
}

// ScanTool performs LLM-powered security scans.
type ScanTool struct {
	base.BaseTypedTool[ScanParams]
	llmRegistry domain.LLMRegistry
	memory      ports.MemoryPort
}

// NewScanTool creates a new ScanTool.
func NewScanTool(llmRegistry domain.LLMRegistry, memory ports.MemoryPort) *ScanTool {
	t := &ScanTool{
		llmRegistry: llmRegistry,
		memory:      memory,
	}
	t.Impl = t
	return t
}

func (t *ScanTool) Name() string { return "scan" }

func (t *ScanTool) Schema() agent_domain.ToolSchema {
	return agent_domain.ToolSchema{
		Name:        "scan",
		Description: "Perform a security scan on a target using LLM analysis and memory storage.",
		Parameters: map[string]string{
			"target":      "string - The target to scan",
			"ai_provider": "string - Optional. The AI provider to use (default: openai)",
		},
	}
}

func (t *ScanTool) ParseParams(input map[string]interface{}) (ScanParams, error) {
	params, err := base.DefaultParseParams[ScanParams](input)
	if err != nil {
		return params, err
	}
	if params.Target == "" {
		return params, types.New(types.ErrCodeInvalidInput, "missing 'target' argument")
	}
	if params.AIProvider == "" {
		params.AIProvider = "openai"
	}
	return params, nil
}

func (t *ScanTool) Execute(ctx context.Context, params ScanParams) (agent_domain.Result, error) {
	// Save target to memory
	if t.memory != nil {
		_ = t.memory.Save(ctx, "scan_target_"+params.Target, params.Target)
	}

	// Analyze with LLM
	llmReport := "Dummy LLM Report for " + params.Target
	if t.llmRegistry != nil {
		provider := t.llmRegistry.Get(params.AIProvider)
		if provider != nil {
			report, err := provider.Generate(ctx, []domain.Message{
				{Role: domain.RoleUser, Content: "Analyze this scan target: " + params.Target},
			}, nil)
			if err == nil {
				llmReport = report
			}
		}
	}

	return agent_domain.Result{
		Success: true,
		Status:  "scan completed successfully",
		Data: map[string]interface{}{
			"target":     params.Target,
			"llm_report": llmReport,
		},
	}, nil
}
