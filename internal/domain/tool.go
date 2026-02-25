package domain

import (
	"context"
)

// ToolSchema defines the metadata for a tool, used by LLMs for function calling.
type ToolSchema struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
}

// Tool is the basic, LLM-safe interface for all tools.
// Moved to the execution core to preserve kernel purity and dependency direction.
type Tool interface {
	Name() string
	Schema() ToolSchema
	ExecuteRaw(ctx context.Context, input map[string]interface{}) (Result, error)
}

// TypedTool provides a type-safe interface for tool execution.
type TypedTool[P any] interface {
	Tool
	ParseParams(input map[string]interface{}) (P, error)
	Execute(ctx context.Context, params P) (Result, error)
}
