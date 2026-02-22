package chat

import (
	"context"
	"duckops/internal/domain"
	"duckops/internal/ports"
	"duckops/internal/tools/base"
	"duckops/internal/types"
)

// ChatParams defines the parameters for the chat tool.
type ChatParams struct {
	Prompt     string `json:"prompt"`
	AIProvider string `json:"ai_provider"`
}

// ChatTool implements the base.Tool interface for chatting with an LLM.
type ChatTool struct {
	base.BaseTypedTool[ChatParams]
	llmRegistry ports.LLMRegistry
}

// NewChatTool creates a new instance of ChatTool.
func NewChatTool(llmRegistry ports.LLMRegistry) *ChatTool {
	t := &ChatTool{
		llmRegistry: llmRegistry,
	}
	t.Impl = t
	return t
}

// Name returns the name of the tool.
func (t *ChatTool) Name() string {
	return "chat"
}

// Schema returns the tool schema for LLM function calling.
func (t *ChatTool) Schema() base.ToolSchema {
	return base.ToolSchema{
		Name:        "chat",
		Description: "A tool for chatting with various LLM providers.",
		Parameters: map[string]string{
			"prompt":      "string",
			"ai_provider": "string",
		},
	}
}

// ParseParams parses the raw input into ChatParams.
func (t *ChatTool) ParseParams(input map[string]interface{}) (ChatParams, error) {
	params, err := base.DefaultParseParams[ChatParams](input)
	if err != nil {
		return params, err
	}
	if params.Prompt == "" {
		return params, types.New(types.ErrCodeInvalidInput, "missing 'prompt' in arguments")
	}
	if params.AIProvider == "" {
		params.AIProvider = "gemini" // Default fallback
	}
	return params, nil
}

// Execute performs the chat operation.
func (t *ChatTool) Execute(ctx context.Context, params ChatParams) (domain.Result, error) {
	llm := t.llmRegistry.Get(params.AIProvider)
	if llm == nil {
		return domain.Result{
			Success: false,
			Error:   "provider not found",
		}, types.Newf(types.ErrCodeNotFound, "LLM provider '%s' not found", params.AIProvider)
	}

	response, err := llm.Generate(ctx, []ports.Message{
		{Role: ports.RoleUser, Content: params.Prompt},
	}, nil)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   err.Error(),
		}, types.Wrapf(err, types.ErrCodeInternal, "failed to generate response with provider '%s'", params.AIProvider)
	}

	return domain.Result{
		Status:  "success",
		Success: true,
		Data: map[string]interface{}{
			"response": response,
			"provider": params.AIProvider,
		},
	}, nil
}
