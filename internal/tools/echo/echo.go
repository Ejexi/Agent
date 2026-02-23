package echo

import (
	"context"

	"github.com/SecDuckOps/Agent/internal/domain"
	"github.com/SecDuckOps/Agent/internal/tools/base"
)

type EchoParams map[string]interface{}

type EchoTool struct {
	base.BaseTypedTool[EchoParams]
}

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
		Description: "A simple tool that returns the input as result",
		Parameters:  nil,
	}
}

func (t *EchoTool) ParseParams(input map[string]interface{}) (EchoParams, error) {
	return EchoParams(input), nil
}

func (t *EchoTool) Execute(ctx context.Context, params EchoParams) (domain.Result, error) {
	return domain.Result{
		Status:  "done",
		Success: true,
		Data:    map[string]interface{}(params),
	}, nil
}
