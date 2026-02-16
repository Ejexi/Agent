// tool base interface define tools roles
package base

import (
	"context"
	"time"
)

type Tool interface {
	Name() string
	Description() string
	// interface methods
	Execute(ctx context.Context, params ToolParameters) (*ToolResult, error)
	Validate(params ToolParameters) error
}

// contains input data for a tool
type ToolParameters struct {
	InputData map[string]interface{} //key,value input
}

// contains the output from a tool execution
type ToolResult struct {
	Success  bool
	Data     interface{}
	Error    error
	Duration time.Duration
}

// provides common functionality for all tools
type BaseTool struct {
	name        string
	description string
}

func NewBaseTool(name, description string) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
	}
}

// returns the tool name
func (b *BaseTool) Name() string {
	return b.name
}

// returns the tool description
func (b *BaseTool) Description() string {
	return b.description
}

// return nothing (placeholder) will be implemented latter withe the tools
func (b *BaseTool) Validate(params ToolParameters) error {
	return nil
}
