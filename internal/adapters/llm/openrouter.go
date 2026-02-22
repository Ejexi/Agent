package llm

import (
	"context"
	"duckops/internal/ports"
	types "duckops/internal/types"
	"net/http"
	"time"

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

	// Reuse HTTP transport for max performance
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	config.HTTPClient = &http.Client{
		Timeout: 60 * time.Second,
		Transport: newHeaderTransport(
			map[string]string{
				"HTTP-Referer": "https://github.com/DuckOps/DuckOps",
				"X-Title":      "DuckOps Agent",

				// Prompt caching headers
				"Cache-Control": "max-age=3600",

				// OpenRouter recommended headers
				"X-OpenRouter-Client": "duckops",

				// Anthropic caching (only used by Claude models)
				"anthropic-beta": "prompt-caching-2024-07-31",
			},
			transport,
		),
	}

	if model == "" {
		model = "meta-llama/llama-3.1-70b-instruct"
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
func (o *OpenRouterAdapter) Generate(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (string, error) {
	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	req := openai.ChatCompletionRequest{
		Model:     o.model,
		Messages:  reqMessages,
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

// Stream implements the LLM Port with streaming support
func (o *OpenRouterAdapter) Stream(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (<-chan ports.ChatChunk, error) {
	ch := make(chan ports.ChatChunk)

	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    o.model,
		Messages: reqMessages,
		Stream:   true,
	}

	stream, err := o.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeToolExecution, "openrouter streaming error")
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

// HealthCheck verifies connectivity to OpenRouter.
func (o *OpenRouterAdapter) HealthCheck(ctx context.Context) error {
	_, err := o.client.GetModel(ctx, o.model)
	return err
}
