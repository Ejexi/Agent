// testing tools
package echo

import (
	"context"
	"fmt"
	"time"

	"github.com/Ejexi/Agent/internal/tools/base"
)

// simple tool that echoes back messages
// testing the tool system
type EchoTool struct {
	*base.BaseTool // Embed BaseTool to get Name() and Description()
}

// New creates a new echo tool
func New() *EchoTool {
	return &EchoTool{ //return pointer object to use it latter
		BaseTool: base.NewBaseTool(
			"echo",
			"Echoes back the input message. We Use this to test if tools are working.",
		),
	}
}

// Execute runs the echo tool
func (t *EchoTool) Execute(ctx context.Context, params base.ToolParameters) (*base.ToolResult, error) {
	start := time.Now()

	// Validate input
	if err := t.Validate(params); err != nil {
		return &base.ToolResult{
			Success: false,
			Error:   err,
		}, err
	}
	// Get the message parameter
	message, ok := params.InputData["message"].(string)
	if !ok {
		err := fmt.Errorf("message parameter is required and must be a string")
		return &base.ToolResult{
			Success: false,
			Error:   err,
		}, err
	}
	// Echo it back
	result := fmt.Sprintf("Echo: %s", message)

	// Return success result
	return &base.ToolResult{
		Success:  true,
		Data:     result,
		Error:    nil,
		Duration: time.Since(start),
	}, nil
}

// Validate checks if the input is valid
func (t *EchoTool) Validate(params base.ToolParameters) error {
	// Check if InputData exists
	if params.InputData == nil {
		return fmt.Errorf("input data is required")
	}

	// Check if message parameter exists
	if _, exists := params.InputData["message"]; !exists {
		return fmt.Errorf("message parameter is required")
	}

	return nil
}
