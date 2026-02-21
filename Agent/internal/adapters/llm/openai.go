package llm

import (
	types "agent/internal/Types"
	"context"

	"github.com/sashabaranov/go-openai"
)

// OpenAIAdapter implements the ports.LLM interface via go-openai.
type OpenAIAdapter struct {
	client *openai.Client
	model  string
}

// NewOpenAIAdapter instantiates the openAI client
func NewOpenAIAdapter(apiKey string, model string) *OpenAIAdapter {
	if model == "" {
		model = openai.GPT4o // Set default
	}

	return &OpenAIAdapter{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

// Name returns the provider identifier string
func (a *OpenAIAdapter) Name() string {
	return "openai"
}

// Generate implements the standard LLM generate interface
func (a *OpenAIAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	}
	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", types.Wrap(err, types.ErrCodeToolExecution, "openai generation failed")
	}

	if len(resp.Choices) == 0 {
		return "", types.New(types.ErrCodeToolExecution, "empty response received from openai")
	}

	return resp.Choices[0].Message.Content, nil
}
