package security

import (
	"context"
	"fmt"
	"strings"

	"github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/ports"
)

// AIReviewer implements ports.ThinkingPort.
// It uses an LLM to generate a rationale or risk assessment for an OS command.
type AIReviewer struct {
	llmRegistry domain.LLMRegistry
	provider    string
	logger      ports.Logger
}

// NewAIReviewer creates a new AI reasoning adapter.
func NewAIReviewer(registry domain.LLMRegistry, provider string, logger ports.Logger) *AIReviewer {
	return &AIReviewer{
		llmRegistry: registry,
		provider:    provider,
		logger:      logger,
	}
}

func (a *AIReviewer) Analyze(ctx context.Context, command string, args []string) (domain.GenerationResult, error) {
	llm := a.llmRegistry.Get(a.provider)
	if llm == nil {
		return domain.GenerationResult{}, fmt.Errorf("LLM provider '%s' not found", a.provider)
	}

	prompt := fmt.Sprintf(`You are the DuckOps Command Reviewer.
Analyze the following OS command and provide a VERY brief (1 sentence) rationale or risk assessment.
Focus on:
- What is the intent? (e.g., "Inspecting file system", "Registering a service")
- Is there any immediate risk? (e.g., "Destructive operation", "Modifies system configuration")

Command: %s %v

Response format: Just the rationale text, no prefix like "Rationale:".`, command, args)

	result, err := llm.Generate(ctx, []domain.Message{
		{Role: domain.RoleUser, Content: prompt},
	}, nil)

	if err != nil {
		return domain.GenerationResult{}, err
	}

	result.Content = strings.TrimSpace(result.Content)
	return result, nil
}

func (a *AIReviewer) Reflect(ctx context.Context, command string, args []string, stdout string, stderr string) (domain.GenerationResult, error) {
	llm := a.llmRegistry.Get(a.provider)
	if llm == nil {
		return domain.GenerationResult{}, fmt.Errorf("LLM provider '%s' not found", a.provider)
	}

	content := stdout
	if content == "" {
		content = stderr
	}
	if content == "" {
		return domain.GenerationResult{Content: "Command executed successfully with no output."}, nil
	}

	// Truncate very long output for the LLM
	if len(content) > 2000 {
		content = content[:2000] + "... [output truncated]"
	}

	prompt := fmt.Sprintf(`You are the DuckOps Output Beautifier.
Your job is to read the raw output of an OS command and present it in a premium, highly readable format.

Command: %s %v
Raw Output:
%s

Instructions:
- Transform the raw output into a clean, structured presentation.
- Use Markdown tables for listings (like file lists or process tables) if it improves clarity.
- Use bolding and lists to highlight key information.
- Provide a very brief summary if the output is dense.
- If the output is an error, explain it simply.

Response format: Just the beautified presentation text, no conversational prefix.`, command, args, content)

	result, err := llm.Generate(ctx, []domain.Message{
		{Role: domain.RoleUser, Content: prompt},
	}, nil)

	if err != nil {
		return domain.GenerationResult{}, err
	}

	result.Content = strings.TrimSpace(result.Content)
	return result, nil
}
