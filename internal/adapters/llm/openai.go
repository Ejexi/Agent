package llm

import (
	"context"
	"duckops/internal/ports"
	types "duckops/internal/types"

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
func (a *OpenAIAdapter) Generate(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (string, error) {
	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	req := openai.ChatCompletionRequest{
		Model:     a.model,
		Messages:  reqMessages,
		MaxTokens: 5000,
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

// Stream implements the LLM Port with streaming support
func (a *OpenAIAdapter) Stream(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (<-chan ports.ChatChunk, error) {
	ch := make(chan ports.ChatChunk)

	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    a.model,
		Messages: reqMessages,
		Stream:   true,
	}

	stream, err := a.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeToolExecution, "openai streaming error")
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

// HealthCheck verifies connectivity to OpenAI.
func (a *OpenAIAdapter) HealthCheck(ctx context.Context) error {
	_, err := a.client.GetModel(ctx, a.model)
	return err
}
