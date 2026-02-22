package llm

import (
	types "duckops/internal/types"
	"context"

	"github.com/sashabaranov/go-openai"
)

// LMStudioAdapter implements ports.LLM via the LM Studio local API proxy.
type LMStudioAdapter struct {
	client *openai.Client
	model  string
}

// NewLMStudioAdapter instantiates the LM Studio client
func NewLMStudioAdapter(apiKey string, model string, baseURL string) *LMStudioAdapter {
	if apiKey == "" {
		apiKey = "not-needed" // LM Studio doesn't enforce API keys usually
	}

	config := openai.DefaultConfig(apiKey)

	// Override the BaseURL to point to LM Studio's local server
	if baseURL == "" {
		baseURL = "http://localhost:1234/v1"
	}
	config.BaseURL = baseURL

	if model == "" {
		model = "local-model" // LM Studio usually uses whatever model is currently loaded
	}

	return &LMStudioAdapter{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}
}

// Name identifies this LLM port
func (l *LMStudioAdapter) Name() string {
	return "lmstudio"
}

// Generate implements the LLM Port using OpenAI's compatible completion struct
func (l *LMStudioAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: l.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens: 4000,
	}

	resp, err := l.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", types.Wrap(err, types.ErrCodeToolExecution, "lmstudio generation failed")
	}

	if len(resp.Choices) == 0 {
		return "", types.New(types.ErrCodeToolExecution, "empty response received from lmstudio")
	}

	return resp.Choices[0].Message.Content, nil
}
