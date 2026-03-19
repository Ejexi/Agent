// Package mcp defines domain types for Model Context Protocol server integration.
// MCP servers expose tools that DuckOps consumes exactly like built-in tools —
// the LLM sees them in its tool list and calls them by name.
package mcp

// ServerConfig defines how to connect to one MCP server.
// Loaded from ~/.duckops/config.toml under [[mcp.servers]].
type ServerConfig struct {
	// Name is the unique identifier for this server (e.g. "github", "filesystem").
	Name string `toml:"name" json:"name"`

	// Transport is "stdio" or "sse".
	//   stdio: launch a child process, communicate over stdin/stdout (most common)
	//   sse:   connect to a running HTTP server via Server-Sent Events
	Transport string `toml:"transport" json:"transport"`

	// Command is the executable to launch (stdio only).
	// Example: ["npx", "-y", "@modelcontextprotocol/server-github"]
	Command []string `toml:"command" json:"command,omitempty"`

	// URL is the SSE endpoint (sse only).
	// Example: "http://localhost:3000/sse"
	URL string `toml:"url" json:"url,omitempty"`

	// Env holds extra environment variables to pass to the child process.
	Env map[string]string `toml:"env" json:"env,omitempty"`

	// AllowedTools restricts which tools from this server the agent may call.
	// Empty = all tools exposed by the server are available.
	AllowedTools []string `toml:"allowed_tools" json:"allowed_tools,omitempty"`

	// Enabled toggles the server without removing it from config.
	Enabled bool `toml:"enabled" json:"enabled"`
}

// ToolCall is a request to invoke one MCP tool.
type ToolCall struct {
	ServerName string                 `json:"server_name"`
	ToolName   string                 `json:"tool_name"`
	Arguments  map[string]interface{} `json:"arguments"`
}

// ToolResult is the response from an MCP tool call.
type ToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// ToolInfo describes one tool advertised by an MCP server.
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
	ServerName  string                 `json:"server_name"`
}
