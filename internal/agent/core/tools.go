package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// ═══════════════════════════════════════════════════════════
//              TOOL EXECUTION METHODS
// ═══════════════════════════════════════════════════════════

// executeTool parses tool command and executes the tool
// Input format:  "TOOL:echo|{"message":"hello"}"
// Output: Tool result as string
func (a *Agent) executeTool(ctx context.Context, toolCommand string) (string, error) {
	// Step 1: Parse the tool command
	toolName, params, err := a.parseToolCommand(toolCommand)
	if err != nil {
		return "", fmt.Errorf("failed to parse tool command: %w", err)
	}

	a.logger.Info("Executing tool",
		zap.String("tool", toolName),
		zap.Any("params", params),
	)

	// Step 2: Get tool from registry
	tool, err := a.registry.Get(toolName)
	if err != nil {
		return "", fmt.Errorf("tool '%s' not found: %w", toolName, err)
	}

	// Step 3: Execute tool with raw params (two-layer abstraction handles validation)
	result, err := tool.ExecuteRaw(ctx, params)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	// Step 4: Check if tool succeeded
	if !result.Success {
		return "", fmt.Errorf("tool returned failure: %v", result.Error)
	}

	// Step 5: Format result
	a.logger.Info("Tool executed successfully",
		zap.String("tool", toolName),
		zap.Duration("duration", result.Duration),
	)

	return fmt.Sprintf("%v", result.Data), nil
}

// parseToolCommand extracts tool name and parameters from command string
// Input:  "TOOL:echo|{"message":"hello"}"
// Output: toolName="echo", params=map{"message":"hello"}
func (a *Agent) parseToolCommand(command string) (string, map[string]interface{}, error) {
	// Remove "TOOL:" prefix
	command = strings.TrimPrefix(command, "TOOL:")
	command = strings.TrimSpace(command)

	// Split by "|" to separate tool name and params
	// "echo|{"message":"hello"}" → ["echo", "{\"message\":\"hello\"}"]
	parts := strings.SplitN(command, "|", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid format: expected 'TOOL:name|{json}', got '%s'", command)
	}

	toolName := strings.TrimSpace(parts[0])
	paramsJSON := strings.TrimSpace(parts[1])

	// Validate tool name
	if toolName == "" {
		return "", nil, fmt.Errorf("tool name cannot be empty")
	}

	// Parse JSON parameters
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return "", nil, fmt.Errorf("invalid JSON parameters: %w (got: %s)", err, paramsJSON)
	}

	return toolName, params, nil
}
