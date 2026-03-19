package mcp_tool

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/tools/base"
)

// MCPListParams — no parameters needed.
type MCPListParams struct{}

// MCPListTool lets the LLM discover all available MCP servers and their tools.
type MCPListTool struct {
	base.BaseTypedTool[MCPListParams]
	client ports.MCPClientPort
}

func NewMCPListTool(client ports.MCPClientPort) *MCPListTool {
	t := &MCPListTool{client: client}
	t.Impl = t
	return t
}

func (t *MCPListTool) ParseParams(input map[string]interface{}) (MCPListParams, error) {
	return base.DefaultParseParams[MCPListParams](input)
}

func (t *MCPListTool) Name() string { return "mcp_list" }

func (t *MCPListTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "mcp_list",
		Description: "List all connected MCP servers and the tools they expose. Call this first to discover what's available before calling mcp_call.",
		Parameters:  map[string]string{},
	}
}

func (t *MCPListTool) Execute(ctx context.Context, _ MCPListParams) (domain.Result, error) {
	servers := t.client.ConnectedServers()
	serverTools := make(map[string]interface{}, len(servers))

	for _, srv := range servers {
		tools, err := t.client.ListServerTools(ctx, srv)
		if err != nil {
			serverTools[srv] = map[string]string{"error": err.Error()}
			continue
		}
		toolList := make([]map[string]string, len(tools))
		for i, tool := range tools {
			toolList[i] = map[string]string{
				"name":        tool.Name,
				"description": tool.Description,
			}
		}
		serverTools[srv] = toolList
	}

	return domain.Result{
		Success: true,
		Status:  "mcp servers listed",
		Data: map[string]interface{}{
			"connected_servers": servers,
			"tools_by_server":   serverTools,
		},
	}, nil
}
