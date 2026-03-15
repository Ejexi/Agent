package reporting

import (
	"context"
	"fmt"

	"github.com/SecDuckOps/agent/internal/domain"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/agent/internal/tools/base"
)

type ReportingTool struct {
	base.BaseTypedTool[ReportingParams]
	llmRegistry shared_domain.LLMRegistry
}

type ReportingParams struct {
	Data   string `json:"data"`
	Format string `json:"format"` // e.g., "ExecutiveSummary", "DetailedFindings", "CleanupGuide"
}

func NewReportingTool(llmRegistry shared_domain.LLMRegistry) *ReportingTool {
	t := &ReportingTool{
		llmRegistry: llmRegistry,
	}
	t.Impl = t
	return t
}

func (t *ReportingTool) Name() string {
	return "generate_report"
}

func (t *ReportingTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "generate_report",
		Description: "Transforms raw scanning data or investigation findings into a structured Markdown report using AI. Use this to summarize complex results for the user.",
		Parameters: map[string]string{
			"data":   "The raw logs, findings, or data to process.",
			"format": "The specialized reporting format: 'ExecutiveSummary', 'DetailedFindings', or 'CleanupGuide'.",
		},
	}
}

func (t *ReportingTool) ParseParams(input map[string]interface{}) (ReportingParams, error) {
	return base.DefaultParseParams[ReportingParams](input)
}

func (t *ReportingTool) Execute(ctx context.Context, params ReportingParams) (domain.Result, error) {
	if t.llmRegistry == nil {
		return domain.Result{
			Success: false,
			Error:   "LLM registry not available for reporting",
		}, nil
	}

	// Use a default model for reporting (preference for high intelligence)
	providers := t.llmRegistry.List()
	if len(providers) == 0 {
		return domain.Result{Success: false, Error: "No LLM providers"}, nil
	}
	
	llm := t.llmRegistry.Get(providers[0]) // Get primary provider
	
	systemPrompt := fmt.Sprintf(`You are a Professional DevSecOps Reporter. 
Your task is to transform the provided raw data into a structured Markdown report in the format: %s.
Rules:
1. Deduplicate findings.
2. Group related issues.
3. Use clear Markdown headers and tables.
4. Highlight critical severity items first.
5. Do not include fluff, focus on technical accuracy and remediation.`, params.Format)

	messages := []shared_domain.Message{
		{Role: shared_domain.RoleSystem, Content: systemPrompt},
		{Role: shared_domain.RoleUser, Content: fmt.Sprintf("RAW FINDINGS:\n%s", params.Data)},
	}

	result, err := llm.Generate(ctx, messages, nil)
	if err != nil {
		return domain.Result{Success: false, Error: err.Error()}, err
	}

	return domain.Result{
		Success: true,
		Data: map[string]interface{}{
			"report": result.Content,
			"format": params.Format,
		},
	}, nil
}
