package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain/mcp"
)

// MCPClientPort is the interface for communicating with MCP servers.
// The Kernel and SessionActor use this to call tools exposed by external servers.
type MCPClientPort interface {
	// CallTool invokes one tool on the named MCP server.
	CallTool(ctx context.Context, call mcp.ToolCall) (mcp.ToolResult, error)

	// ListTools returns all tools available across all connected servers.
	// Filtered by AllowedTools if configured per server.
	ListTools(ctx context.Context) ([]mcp.ToolInfo, error)

	// ListServerTools returns tools from one specific server.
	ListServerTools(ctx context.Context, serverName string) ([]mcp.ToolInfo, error)

	// IsConnected reports whether the named server is alive.
	IsConnected(serverName string) bool

	// ConnectedServers returns the names of all currently connected servers.
	ConnectedServers() []string

	// Close shuts down all server connections cleanly.
	Close() error
}
