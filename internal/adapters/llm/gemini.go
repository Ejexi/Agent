package llm

import (
	"context"
	"duckops/internal/ports"
	types "duckops/internal/types"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
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
func (g *GeminiAdapter) Generate(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	modelName := g.model
	if opts != nil && opts.Model != "" {
		modelName = opts.Model
	}

	model := g.client.GenerativeModel(modelName)
	cs := model.StartChat()

	// Map history (all but last message)
	if len(messages) > 1 {
		history := messages[:len(messages)-1]
		genaiHistory := make([]*genai.Content, len(history))
		for i, msg := range history {
			role := "user"
			if msg.Role == ports.RoleAssistant {
				role = "model"
			}
			genaiHistory[i] = &genai.Content{
				Role:  role,
				Parts: []genai.Part{genai.Text(msg.Content)},
			}
		}
		cs.History = genaiHistory
	}

	lastMsg := messages[len(messages)-1]
	resp, err := cs.SendMessage(ctx, genai.Text(lastMsg.Content))
	if err != nil {
		return "", types.Wrap(err, types.ErrCodeToolExecution, "failed to generate from gemini API")
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", types.New(types.ErrCodeToolExecution, "empty response generated from gemini")
	}

	fullText := ""
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			fullText += string(text)
		}
	}

	return fullText, nil
}

// Stream implements the LLM Port with streaming support
func (g *GeminiAdapter) Stream(ctx context.Context, messages []ports.Message, opts *ports.GenerateOptions) (<-chan ports.ChatChunk, error) {
	if len(messages) == 0 {
		return nil, types.New(types.ErrCodeInvalidInput, "no messages provided for streaming")
	}

	modelName := g.model
	if opts != nil && opts.Model != "" {
		modelName = opts.Model
	}

	ch := make(chan ports.ChatChunk)
	model := g.client.GenerativeModel(modelName)
	cs := model.StartChat()

	// Map history
	if len(messages) > 1 {
		history := messages[:len(messages)-1]
		genaiHistory := make([]*genai.Content, len(history))
		for i, msg := range history {
			role := "user"
			if msg.Role == ports.RoleAssistant {
				role = "model"
			}
			genaiHistory[i] = &genai.Content{
				Role:  role,
				Parts: []genai.Part{genai.Text(msg.Content)},
			}
		}
		cs.History = genaiHistory
	}

	lastMsg := messages[len(messages)-1]
	iter := cs.SendMessageStream(ctx, genai.Text(lastMsg.Content))

	go func() {
		defer close(ch)
		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				return
			}
			if err != nil {
				ch <- ports.ChatChunk{Error: err}
				return
			}

			if len(resp.Candidates) > 0 {
				for _, part := range resp.Candidates[0].Content.Parts {
					if text, ok := part.(genai.Text); ok {
						ch <- ports.ChatChunk{Content: string(text)}
					}
				}
			}
		}
	}()

	return ch, nil
}

// HealthCheck verifies connectivity to Gemini.
func (g *GeminiAdapter) HealthCheck(ctx context.Context) error {
	// Simple check by listing models
	iter := g.client.ListModels(ctx)
	_, err := iter.Next()
	if err == iterator.Done {
		return nil
	}
	return err
}

// Close is specific to gemini adapter to clean up network connections on shutdown.
func (g *GeminiAdapter) Close() {
	if g.client != nil {
		g.client.Close()
	}
}
