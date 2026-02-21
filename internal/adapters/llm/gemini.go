package llm

import (
	types "agent/internal/Types"
	"context"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiAdapter implements ports.LLM via the native generative-ai-go SDK.
type GeminiAdapter struct {
	client *genai.Client
	model  string
}

// NewGeminiAdapter instantiates a persistent Gemini client for high performance.
func NewGeminiAdapter(ctx context.Context, apiKey string, model string) (*GeminiAdapter, error) {
	if model == "" {
		model = "gemini-1.5-flash"
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to initialize persistent gemini client")
	}

	return &GeminiAdapter{
		client: client, // Reused across all tool executions!
		model:  model,
	}, nil
}

// Name identifies this LLM port
func (g *GeminiAdapter) Name() string {
	return "gemini"
}

// Generate uses the persistent client, eliminating setup/teardown latency.
func (g *GeminiAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	model := g.client.GenerativeModel(g.model)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", types.Wrap(err, types.ErrCodeToolExecution, "failed to generate from gemini API")
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", types.New(types.ErrCodeToolExecution, "empty response generated from gemini")
	}

	part := resp.Candidates[0].Content.Parts[0]
	if text, ok := part.(genai.Text); ok {
		return string(text), nil
	}

	return "", types.New(types.ErrCodeToolExecution, "unexpected gemini data type returned")
}

// Close is specific to gemini adapter to clean up network connections on shutdown.
func (g *GeminiAdapter) Close() {
	if g.client != nil {
		g.client.Close()
	}
}
