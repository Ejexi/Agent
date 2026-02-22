package llm

import (
	"context"
	"duckops/internal/ports"
	"duckops/internal/types"
	"net/http"

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

	// Add generic caching headers
	config.HTTPClient = &http.Client{
		Transport: newHeaderTransport(map[string]string{
			"anthropic-beta": "prompt-caching-2024-07-31",
		}, nil),
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
func (a *OpenAICompatibleAdapter) Generate(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (string, error) {
	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	model := a.model
	if opts != nil && opts.Model != "" {
		model = opts.Model
	}

	req := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    reqMessages,
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	if opts != nil {
		if opts.MaxTokens > 0 {
			req.MaxTokens = opts.MaxTokens
		}
		if opts.Temperature > 0 {
			req.Temperature = opts.Temperature
		}
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

// Stream implements the ports.LLM interface.
func (a *OpenAICompatibleAdapter) Stream(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (<-chan ports.ChatChunk, error) {
	ch := make(chan ports.ChatChunk)

	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	model := a.model
	if opts != nil && opts.Model != "" {
		model = opts.Model
	}

	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: reqMessages,
		Stream:   true,
	}

	if opts != nil {
		if opts.MaxTokens > 0 {
			req.MaxTokens = opts.MaxTokens
		}
	}

	stream, err := a.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, types.Wrapf(err, types.ErrCodeToolExecution, "%s streaming error", a.Name())
	}

	go func() {
		defer close(ch)
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					return
				}
				ch <- ports.ChatChunk{Error: err}
				return
			}

			if len(response.Choices) > 0 {
				content := response.Choices[0].Delta.Content
				if content != "" {
					ch <- ports.ChatChunk{Content: content}
				}
			}
		}
	}()

	return ch, nil
}

// HealthCheck verifies connectivity to the provider.
func (a *OpenAICompatibleAdapter) HealthCheck(ctx context.Context) error {
	// Simple check by listing models or just a ping if supported
	_, err := a.client.GetModel(ctx, a.model)
	return err
}
