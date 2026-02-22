package ports

import "context"

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type Message struct {
	Role    MessageRole `json:"role"`
	Content string      `json:"content"`

	// Optional: used for tool calling and advanced providers
	Name string `json:"name,omitempty"`
}

type GenerateOptions struct {
	Model       string
	MaxTokens   int
	Temperature float32
	TopP        float32
}

type ChatChunk struct {
	Content string
	Error   error
	Done    bool
}

type LLM interface {

	// Provider name (openai, ollama, openrouter, etc)
	Name() string

	// Generate full response (blocking)
	Generate(
		ctx context.Context,
		messages []Message,
		opts *GenerateOptions,
	) (string, error)

	// Stream response (non-blocking, low latency)
	Stream(
		ctx context.Context,
		messages []Message,
		opts *GenerateOptions,
	) (<-chan ChatChunk, error)

	// Optional but very useful
	HealthCheck(ctx context.Context) error
}

type LLMRegistry interface {
	Register(llm LLM)

	Get(name string) LLM

	MustGet(name string) LLM

	List() []string

	Default() LLM
}