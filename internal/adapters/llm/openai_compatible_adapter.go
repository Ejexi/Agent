package llm

import (
	"context"
	"duckops/internal/ports"
	"duckops/internal/types"

	"github.com/sashabaranov/go-openai"
)

// OpenAICompatibleAdapter implements the ports.LLM interface for any provider
// that supports the OpenAI API specification (e.g., Ollama, vLLM, Groq, etc.).
type OpenAICompatibleAdapter struct {
	providerName string
	client       *openai.Client
	model        string
}

// NewOpenAICompatibleAdapter initializes a generic OpenAI-compatible client.
// It accepts a custom base URL to point to any compatible endpoint.
func NewOpenAICompatibleAdapter(name, apiKey, model, baseURL string) ports.LLM {
	// 1. Configure the OpenAI client with a custom BaseURL
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	// 2. Return the adapter which satisfies ports.LLM
	return &OpenAICompatibleAdapter{
		providerName: name,
		client:       openai.NewClientWithConfig(config),
		model:        model,
	}
}

// Name returns the identifier for this provider.
func (a *OpenAICompatibleAdapter) Name() string {
	return a.providerName
}

// Generate implements the ports.LLM interface.
// While the interface currently supports a single prompt string,
// the adapter is built using the ChatCompletion API to support modern models.
func (a *OpenAICompatibleAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	// Example of supporting system prompts by prepending if needed,
	// though here we follow the standard user message pattern.
	req := openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		// You can add more production-grade defaults here
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", types.Wrapf(err, types.ErrCodeToolExecution, "%s provider error", a.Name())
	}

	if len(resp.Choices) == 0 {
		return "", types.Newf(types.ErrCodeToolExecution, "received empty response from %s", a.Name())
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateChat is an extended method (not currently in ports.LLM) that demonstrates
// how this adapter handles full chat history (System, User, Assistant).
func (a *OpenAICompatibleAdapter) GenerateChat(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	req := openai.ChatCompletionRequest{
		Model:    a.model,
		Messages: messages,
	}

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", types.Wrapf(err, types.ErrCodeToolExecution, "%s chat error", a.Name())
	}

	if len(resp.Choices) == 0 {
		return "", types.Newf(types.ErrCodeToolExecution, "received empty response from %s", a.Name())
	}

	return resp.Choices[0].Message.Content, nil
}
