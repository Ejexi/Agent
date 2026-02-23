package base

import (
	"context"
	"github.com/SecDuckOps/Agent/internal/domain"
	"encoding/json"
	"fmt"
)

// ToolSchema defines the metadata for a tool, used by LLMs for function calling.
type ToolSchema struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
}

// Tool is the basic, LLM-safe interface for all tools.
type Tool interface {
	Name() string
	Schema() ToolSchema
	ExecuteRaw(ctx context.Context, input map[string]interface{}) (domain.Result, error)
}

// TypedTool provides a type-safe interface for tool execution.
type TypedTool[P any] interface {
	Tool
	ParseParams(input map[string]interface{}) (P, error)
	Execute(ctx context.Context, params P) (domain.Result, error)
}

// BaseTypedTool is a helper struct that implements ExecuteRaw for TypedTools.
type BaseTypedTool[P any] struct {
	Impl TypedTool[P]
}

// ExecuteRaw handles the conversion from map[string]interface{} to the typed parameter P.
func (b *BaseTypedTool[P]) ExecuteRaw(ctx context.Context, input map[string]interface{}) (domain.Result, error) {
	params, err := b.Impl.ParseParams(input)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   fmt.Sprintf("failed to parse parameters: %v", err),
		}, nil
	}

	return b.Impl.Execute(ctx, params)
}

// DefaultParseParams provides a default implementation using JSON marshalling.
func DefaultParseParams[P any](input map[string]interface{}) (P, error) {
	var params P
	data, err := json.Marshal(input)
	if err != nil {
		return params, err
	}
	err = json.Unmarshal(data, &params)
	return params, err
}
