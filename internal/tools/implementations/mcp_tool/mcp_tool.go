// Package mcp_tool provides an agent tool that proxies calls to MCP servers.
// The LLM can call any tool on any connected MCP server using this single tool,
// or each MCP tool can be registered individually in the kernel tool registry.
package mcp_tool

import (
	"context"
	"fmt"

	"github.com/SecDuckOps/agent/internal/domain"
	mcp_domain "github.com/SecDuckOps/agent/internal/domain/mcp"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/tools/base"
)

// MCPToolParams are the parameters for a direct MCP tool call.
type MCPToolParams struct {
	Server    string                 `json:"server"`
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments"`
}

// MCPTool is a kernel tool that proxies calls to MCP servers.
// Registered once in the kernel, it enables the LLM to reach any MCP tool.
type MCPTool struct {
	base.BaseTypedTool[MCPToolParams]
	client ports.MCPClientPort
}

func NewMCPTool(client ports.MCPClientPort) *MCPTool {
	t := &MCPTool{client: client}
	t.Impl = t
	return t
}

func (t *MCPTool) ParseParams(input map[string]interface{}) (MCPToolParams, error) {
	return base.DefaultParseParams[MCPToolParams](input)
}

func (t *MCPTool) Name() string { return "mcp_call" }

func (t *MCPTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "mcp_call",
		Description: `Call a tool on a connected MCP server.
Use this to access external capabilities like GitHub, filesystem operations, databases, and more.
List available servers and tools with the mcp_list tool first.`,
		Parameters: map[string]string{
			"server":    "string — name of the MCP server (e.g. 'github', 'filesystem')",
			"tool":      "string — tool name on that server",
			"arguments": "object — tool-specific arguments",
		},
	}
}

func (t *MCPTool) Execute(ctx context.Context, params MCPToolParams) (domain.Result, error) {
	if !t.client.IsConnected(params.Server) {
		connected := t.client.ConnectedServers()
		return domain.Result{
			Success: false,
			Status:  "mcp server not connected",
			Data: map[string]interface{}{
				"error":             fmt.Sprintf("MCP server %q is not connected", params.Server),
				"connected_servers": connected,
			},
		}, nil
	}

	result, err := t.client.CallTool(ctx, mcp_domain.ToolCall{
		ServerName: params.Server,
		ToolName:   params.Tool,
		Arguments:  params.Arguments,
	})
	if err != nil {
		return domain.Result{
			Success: false,
			Status:  "mcp call failed",
			Data:    map[string]interface{}{"error": err.Error()},
		}, nil
	}

	return domain.Result{
		Success: !result.IsError,
		Status:  "mcp call completed",
		Data: map[string]interface{}{
			"content":  result.Content,
			"is_error": result.IsError,
		},
	}, nil
}
