package echo

import (
	"context"
	"github.com/SecDuckOps/Agent/internal/domain"
	"github.com/SecDuckOps/Agent/internal/tools/base"
)

// EchoParams defines the input for the echo tool.
type EchoParams struct {
	Message string `json:"message"`
}

// EchoTool is a simple tool that returns the input message.
type EchoTool struct {
	base.BaseTypedTool[EchoParams]
}

// NewEchoTool creates a new instance of the echo tool.
func NewEchoTool() *EchoTool {
	t := &EchoTool{}
	t.Impl = t
	return t
}

func (t *EchoTool) Name() string {
	return "echo"
}

func (t *EchoTool) Schema() base.ToolSchema {
	return base.ToolSchema{
		Name:        "echo",
		Description: "A simple tool that returns the provided message.",
		Parameters: map[string]string{
			"message": "string",
		},
	}
}

func (t *EchoTool) ParseParams(input map[string]interface{}) (EchoParams, error) {
	return base.DefaultParseParams[EchoParams](input)
}

func (t *EchoTool) Execute(ctx context.Context, params EchoParams) (domain.Result, error) {
	return domain.Result{
		Success: true,
		Status:  "completed",
		Data: map[string]interface{}{
			"message": params.Message,
		},
	}, nil
}
