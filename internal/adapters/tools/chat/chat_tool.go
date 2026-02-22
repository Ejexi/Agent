package chat

import (
	"agent/internal/domain"
	"agent/internal/ports"
	"agent/internal/types"
	"context"
)

// ChatTool implements the Tool interface for chatting with an LLM.
type ChatTool struct {
	llmRegistry ports.LLMRegistry
}

// NewChatTool creates a new instance of ChatTool.
func NewChatTool(llmRegistry ports.LLMRegistry) *ChatTool {
	return &ChatTool{
		llmRegistry: llmRegistry,
	}
}

// Name returns the name of the tool, matching what the CLI requests.
func (t *ChatTool) Name() string {
	return "chat"
}

// Run executes the chat tool via the kernel.
func (t *ChatTool) Run(ctx context.Context, task domain.Task) (domain.Result, error) {
	promptInter, ok := task.Args["prompt"]
	if !ok {
		return domain.Result{TaskID: task.ID, Success: false, Error: "missing 'prompt' in arguments"}, types.New(types.ErrCodeInvalidInput, "missing 'prompt' in arguments")
	}

	prompt, ok := promptInter.(string)
	if !ok || prompt == "" {
		return domain.Result{TaskID: task.ID, Success: false, Error: "'prompt' must be a non-empty string"}, types.New(types.ErrCodeInvalidInput, "'prompt' must be a non-empty string")
	}

	providerName := "gemini" // Default fallback
	if provIter, ok := task.Args["ai_provider"]; ok {
		if provStr, ok := provIter.(string); ok && provStr != "" {
			providerName = provStr
		}
	}

	llm := t.llmRegistry.Get(providerName)
	if llm == nil {
		return domain.Result{TaskID: task.ID, Success: false, Error: "provider not found"}, types.Newf(types.ErrCodeNotFound, "LLM provider '%s' not found", providerName)
	}

	response, err := llm.Generate(ctx, prompt)
	if err != nil {
		return domain.Result{TaskID: task.ID, Success: false, Error: err.Error()}, types.Wrapf(err, types.ErrCodeInternal, "failed to generate response with provider '%s'", providerName)
	}

	return domain.Result{
		TaskID:  task.ID,
		Status:  "success",
		Success: true,
		Data: map[string]interface{}{
			"response": response,
			"provider": providerName,
		},
	}, nil
}
