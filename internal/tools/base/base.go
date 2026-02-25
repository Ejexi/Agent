package base

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SecDuckOps/agent/internal/domain"
)

// BaseTypedTool is a helper struct that implements ExecuteRaw for TypedTools.
// This is an implementation helper for extensions, not a core interface.
type BaseTypedTool[P any] struct {
	Impl domain.TypedTool[P]
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
