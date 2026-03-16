package taskengine

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
)

// ThinkingMiddleware generates a rationale using an AI before proceeding.
func ThinkingMiddleware(reviewer ports.ThinkingPort) TaskMiddleware {
	return func(next TaskHandler) TaskHandler {
		return func(ctx context.Context, task *domain.OSTask) domain.OSTaskResult {
			var middleUsage shared_domain.TokenUsage
			var middleModel string

			if reviewer != nil && task.Rationale == "" {
				result, err := reviewer.Analyze(ctx, task.OriginalCmd, task.Args)
				if err == nil {
					// Enrich the task with the rationale
					task.Rationale = result.Content
					middleUsage = result.Usage
					middleModel = "" 
				}
			}
			res := next(ctx, task)
			res.Rationale = task.Rationale
			
			// Accumulate usage
			res.Usage.PromptTokens += middleUsage.PromptTokens
			res.Usage.CompletionTokens += middleUsage.CompletionTokens
			res.Usage.TotalTokens += middleUsage.TotalTokens
			if middleModel != "" {
				res.Model = middleModel
			}
			
			return res
		}
	}
}

// ReflectionMiddleware uses an AI to process the output of a command after it has run.
func ReflectionMiddleware(reviewer ports.ThinkingPort) TaskMiddleware {
	return func(next TaskHandler) TaskHandler {
		return func(ctx context.Context, task *domain.OSTask) domain.OSTaskResult {
			res := next(ctx, task)

			if reviewer != nil && res.Status == domain.StatusCompleted {
				result, err := reviewer.Reflect(ctx, task.OriginalCmd, task.Args, res.Stdout, res.Stderr)
				if err == nil {
					res.Reflection = result.Content
					res.Usage.PromptTokens += result.Usage.PromptTokens
					res.Usage.CompletionTokens += result.Usage.CompletionTokens
					res.Usage.TotalTokens += result.Usage.TotalTokens
					// Don't overwrite model if already set, but set if empty
					if res.Model == "" {
						res.Model = "" 
					}
				}
			}

			return res
		}
	}
}
