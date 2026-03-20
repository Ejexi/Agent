package base

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

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

// GenerateSchemaParams dynamically creates a schema map reading `json` and `desc` tags from a struct.
func GenerateSchemaParams(v interface{}) map[string]string {
	params := make(map[string]string)
	val := reflect.TypeOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return params
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		
		name := strings.Split(jsonTag, ",")[0]
		
		desc := field.Tag.Get("desc")
		if desc != "" {
			params[name] = desc
		} else {
			params[name] = field.Type.String() // fallback
		}
	}
	return params
}
