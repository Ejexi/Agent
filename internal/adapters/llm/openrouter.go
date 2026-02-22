package llm

import (
	types "duckops/internal/types"
	"context"

	"github.com/sashabaranov/go-openai"
)

// OpenRouterAdapter implements ports.LLM via the OpenRouter API proxy.
type OpenRouterAdapter struct {
	client *openai.Client
	model  string
}

// NewOpenRouterAdapter instantiates the OpenRouter client
func NewOpenRouterAdapter(apiKey string, model string) *OpenRouterAdapter {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"

	if model == "" {
		model = "arcee-ai/trinity-large-preview:free" // Example default fallback
	}

	return &OpenRouterAdapter{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}
}

// Name identifies this LLM port
func (o *OpenRouterAdapter) Name() string {
	return "openrouter"
}

// Generate implements the LLM Port using OpenAI's compatible completion struct
func (o *OpenRouterAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens: 5000,
	}
	resp, err := o.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", types.Wrap(err, types.ErrCodeToolExecution, "openrouter generation failed")
	}

	if len(resp.Choices) == 0 {
		return "", types.New(types.ErrCodeToolExecution, "empty response received from openrouter")
	}

	return resp.Choices[0].Message.Content, nil
}
