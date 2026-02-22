package llm

import (
	"context"
	"duckops/internal/ports"
	types "duckops/internal/types"

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
func (l *LMStudioAdapter) Generate(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (string, error) {
	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	req := openai.ChatCompletionRequest{
		Model:     l.model,
		Messages:  reqMessages,
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

// Stream implements the LLM Port with streaming support
func (l *LMStudioAdapter) Stream(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (<-chan ports.ChatChunk, error) {
	ch := make(chan ports.ChatChunk)

	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    l.model,
		Messages: reqMessages,
		Stream:   true,
	}

	stream, err := l.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeToolExecution, "lmstudio streaming error")
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

// HealthCheck verifies connectivity to LM Studio.
func (l *LMStudioAdapter) HealthCheck(ctx context.Context) error {
	// LM Studio doesn't always support GetModel, so we might just check if server is up
	_, err := l.client.ListModels(ctx)
	return err
}
