package security

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/types"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/kernel"
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
		llm = a.llmRegistry.Default()
	}
	if llm == nil {
		return domain.GenerationResult{}, types.New(types.ErrCodeNotFound, "no LLM provider found")
	}

	prompt := fmt.Sprintf(`You are the DuckOps Command Reviewer.
Analyze the following OS command and provide a VERY brief (1 sentence) rationale or risk assessment.
Focus on:
- What is the intent? (e.g., "Inspecting file system", "Registering a service")
- Is there any immediate risk? (e.g., "Destructive operation", "Modifies system configuration")

Command: %s %v

Response format: Just the rationale text, no prefix like "Rationale:".`, command, args)

	var result domain.GenerationResult
	var err error
	maxRetries := 2
	execCtx, _ := kernel.FromContext(ctx)

	for retry := 0; retry <= maxRetries; retry++ {
		ch, streamErr := llm.Stream(ctx, []domain.Message{
			{Role: domain.RoleUser, Content: prompt},
		}, nil)
		
		if streamErr == nil {
			err = nil
			for chunk := range ch {
				if chunk.Error != nil {
					err = chunk.Error
					break
				}
				result.Content += chunk.Content
				if execCtx != nil && chunk.Content != "" {
					execCtx.Emit(agent_domain.ThoughtChunkEvent{Chunk: chunk.Content})
				}
				result.Usage.PromptTokens += chunk.Usage.PromptTokens
				result.Usage.CompletionTokens += chunk.Usage.CompletionTokens
				result.Usage.TotalTokens += chunk.Usage.TotalTokens
			}
			if err == nil {
				break
			}
		} else {
			err = streamErr
		}
		if strings.Contains(err.Error(), "429") && retry < maxRetries {
			select {
			case <-ctx.Done():
				return domain.GenerationResult{}, ctx.Err()
			case <-time.After(time.Duration(1<<retry) * time.Second):
				continue
			}
		}
		break
	}

	if err != nil {
		// Fallback rotation
		providers := a.llmRegistry.List()
		tried := make(map[string]bool)
		tried[llm.Name()] = true

		for _, pName := range providers {
			if tried[pName] {
				continue
			}
			fallbackLLM := a.llmRegistry.Get(pName)
			if fallbackLLM == nil {
				continue
			}

			ch, streamErr := fallbackLLM.Stream(ctx, []domain.Message{
				{Role: domain.RoleUser, Content: prompt},
			}, nil)
			if streamErr == nil {
				var streamFail error
				for chunk := range ch {
					if chunk.Error != nil {
						streamFail = chunk.Error
						break
					}
					result.Content += chunk.Content
					if execCtx != nil && chunk.Content != "" {
						execCtx.Emit(agent_domain.ThoughtChunkEvent{Chunk: chunk.Content})
					}
					result.Usage.PromptTokens += chunk.Usage.PromptTokens
					result.Usage.CompletionTokens += chunk.Usage.CompletionTokens
					result.Usage.TotalTokens += chunk.Usage.TotalTokens
				}
				if streamFail == nil {
					result.Content = strings.TrimSpace(result.Content)
					return result, nil
				}
			}
			tried[pName] = true
		}
		return domain.GenerationResult{}, err
	}

	result.Content = strings.TrimSpace(result.Content)
	return result, nil
}

func (a *AIReviewer) Reflect(ctx context.Context, command string, args []string, stdout string, stderr string) (domain.GenerationResult, error) {
	llm := a.llmRegistry.Get(a.provider)
	if llm == nil {
		llm = a.llmRegistry.Default()
	}
	if llm == nil {
		return domain.GenerationResult{}, types.New(types.ErrCodeNotFound, "no LLM provider found")
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

	var result domain.GenerationResult
	var err error
	maxRetries := 2
	for retry := 0; retry <= maxRetries; retry++ {
		result, err = llm.Generate(ctx, []domain.Message{
			{Role: domain.RoleUser, Content: prompt},
		}, nil)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "429") && retry < maxRetries {
			select {
			case <-ctx.Done():
				return domain.GenerationResult{}, ctx.Err()
			case <-time.After(time.Duration(1<<retry) * time.Second):
				continue
			}
		}
		break
	}

	if err != nil {
		// Fallback rotation
		providers := a.llmRegistry.List()
		tried := make(map[string]bool)
		tried[llm.Name()] = true

		for _, pName := range providers {
			if tried[pName] {
				continue
			}
			fallbackLLM := a.llmRegistry.Get(pName)
			if fallbackLLM == nil {
				continue
			}

			result, err = fallbackLLM.Generate(ctx, []domain.Message{
				{Role: domain.RoleUser, Content: prompt},
			}, nil)
			if err == nil {
				result.Content = strings.TrimSpace(result.Content)
				return result, nil
			}
			tried[pName] = true
		}
		return domain.GenerationResult{}, err
	}

	result.Content = strings.TrimSpace(result.Content)
	return result, nil
}
