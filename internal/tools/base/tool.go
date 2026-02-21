// tool base interface define tools roles
package base

import (
	"context"
	"time"
)

// ================== BASE INTERFACE For Registry =================

// interface that all tools must implement
// Registry stores tools as this interface
type Tool interface {
	Name() string //registry use it to get the name & LLM & Direct access by user
	Description() string
	Schema() ToolSchema
	// interface methods
	ExecuteRaw(ctx context.Context, data map[string]interface{}) (*ToolResult, error)
}

// ============================== TYPED INTERFACE ====================
// adds type safe methods on top of Tool / the result reutrn to *ToolResul
// Individual tools implement this for compile time
type TypedTool[P any] interface {
	Tool // Embed base interface
	Execute(ctx context.Context, params P) (*ToolResult, error)
	ParseParams(data map[string]interface{}) (P, error)
}

// =========================  RESULT TYPES =================================

// contains the output from a tool execution
type ToolResult struct {
	Success  bool
	Data     interface{} // Result data
	Error    error
	Duration time.Duration
}

// ===============  SCHEMA TYPES userd For the LLM Function Calling ==============

// defines the tool for LLM function calling (metadata)
type ToolSchema struct {
	Name        string
	Description string
	Parameters  ParameterSchema
}

// describes all tool parameters
type ParameterSchema struct {
	Type       string                    // JSON object all the time
	Properties map[string]PropertySchema // Parameter definitions
	Required   []string                  // parameter names
}

// describes a single parameter
type PropertySchema struct {
	Type        string
	Description string
	Enum        []string
}

// provides common functionality for all tools
// Embed in tool getting the tool name and discription for free
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
