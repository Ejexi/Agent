// Package mcp provides a client adapter for Model Context Protocol servers.
// Supports two transports:
//   - stdio: spawns a child process, speaks JSON-RPC over stdin/stdout
//   - sse:   connects to a running HTTP server via Server-Sent Events
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/mcp"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/types"
)

// jsonRPCRequest is a JSON-RPC 2.0 request envelope.
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response envelope.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// stdioServer manages one stdio MCP server child process.
type stdioServer struct {
	config mcp.ServerConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
	nextID atomic.Int64
	logger shared_ports.Logger
}

func newStdioServer(cfg mcp.ServerConfig, logger shared_ports.Logger) (*stdioServer, error) {
	if len(cfg.Command) == 0 {
		return nil, types.Newf(types.ErrCodeInvalidInput, "mcp server %q: stdio transport requires command", cfg.Name)
	}

	cmd := exec.Command(cfg.Command[0], cfg.Command[1:]...)

	// Start from parent environment so PATH, HOME, etc. are inherited.
	// Then overlay any server-specific env vars from config.
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "mcp: stdin pipe failed")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "mcp: stdout pipe failed")
	}

	// Capture stderr so the child process doesn't block on a full pipe.
	// We log it at debug level — useful for diagnosing MCP server issues.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "mcp: stderr pipe failed")
	}

	if err := cmd.Start(); err != nil {
		return nil, types.Wrapf(err, types.ErrCodeInternal, "mcp: failed to start server %q", cfg.Name)
	}

	s := &stdioServer{
		config: cfg,
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
		logger: logger,
	}

	// Drain stderr in background — prevents blocking, surfaces errors in logs.
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if logger != nil {
				logger.Info(context.Background(),
					fmt.Sprintf("[mcp/%s] %s", cfg.Name, sc.Text()))
			}
		}
	}()

	// MCP initialisation handshake
	if err := s.initialize(); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}

	return s, nil
}

func (s *stdioServer) initialize() error {
	_, err := s.call("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "duckops",
			"version": "1.0",
		},
	})
	if err != nil {
		return types.Wrapf(err, types.ErrCodeInternal, "mcp: initialize failed for %q", s.config.Name)
	}
	// Send initialized notification (no response expected)
	notif := jsonRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"}
	b, _ := json.Marshal(notif)
	b = append(b, '\n')
	_, _ = s.stdin.Write(b)
	return nil
}

func (s *stdioServer) call(method string, params interface{}) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID.Add(1)
	req := jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "mcp: marshal failed")
	}
	b = append(b, '\n')

	if _, err := s.stdin.Write(b); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "mcp: write to stdin failed")
	}

	// Read response lines until we find the matching ID
	for s.stdout.Scan() {
		line := bytes.TrimSpace(s.stdout.Bytes())
		if len(line) == 0 {
			continue
		}
		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, types.Newf(types.ErrCodeInternal, "mcp rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
	return nil, types.Newf(types.ErrCodeInternal, "mcp: no response for method %q (id %d)", method, id)
}

func (s *stdioServer) listTools() ([]mcp.ToolInfo, error) {
	raw, err := s.call("tools/list", nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "mcp: failed to parse tools list")
	}
	tools := make([]mcp.ToolInfo, len(result.Tools))
	for i, t := range result.Tools {
		tools[i] = mcp.ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			ServerName:  s.config.Name,
		}
	}
	return tools, nil
}

func (s *stdioServer) callTool(toolName string, args map[string]interface{}) (mcp.ToolResult, error) {
	raw, err := s.call("tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return mcp.ToolResult{IsError: true, Content: err.Error()}, nil
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return mcp.ToolResult{IsError: true, Content: "failed to parse tool result"}, nil
	}

	var buf bytes.Buffer
	for _, c := range result.Content {
		if c.Type == "text" {
			buf.WriteString(c.Text)
		}
	}
	return mcp.ToolResult{Content: buf.String(), IsError: result.IsError}, nil
}

func (s *stdioServer) close() {
	_ = s.stdin.Close()
	_ = s.cmd.Process.Kill()
	_ = s.cmd.Wait()
}

// ── Client — manages multiple servers ──────────────────────────────────────

// Client implements ports.MCPClientPort.
// Manages a pool of stdio (and future SSE) MCP servers.
type Client struct {
	servers map[string]*stdioServer // server name → connection
	configs map[string]mcp.ServerConfig
	logger  shared_ports.Logger
	mu      sync.RWMutex
}

// NewClient creates and connects to all enabled MCP servers in the config.
// Servers that fail to start are logged and skipped — they don't block startup.
func NewClient(configs []mcp.ServerConfig, logger shared_ports.Logger) *Client {
	c := &Client{
		servers: make(map[string]*stdioServer),
		configs: make(map[string]mcp.ServerConfig),
		logger:  logger,
	}
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		c.configs[cfg.Name] = cfg
		if cfg.Transport == "stdio" || cfg.Transport == "" {
			srv, err := newStdioServer(cfg, logger)
			if err != nil {
				if logger != nil {
					logger.ErrorErr(context.Background(), err,
						fmt.Sprintf("mcp: failed to connect to server %q — skipping", cfg.Name))
				}
				continue
			}
			c.servers[cfg.Name] = srv
			if logger != nil {
				logger.Info(context.Background(),
					fmt.Sprintf("mcp: connected to server %q (%s)", cfg.Name, cfg.Transport))
			}
		}
		// SSE transport: TODO in a follow-up (requires HTTP client + event stream)
	}
	return c
}

func (c *Client) CallTool(ctx context.Context, call mcp.ToolCall) (mcp.ToolResult, error) {
	c.mu.RLock()
	srv, ok := c.servers[call.ServerName]
	c.mu.RUnlock()

	if !ok {
		return mcp.ToolResult{IsError: true},
			types.Newf(types.ErrCodeNotFound, "mcp server %q not connected", call.ServerName)
	}

	// Respect context deadline
	done := make(chan struct{})
	var result mcp.ToolResult
	var err error
	go func() {
		result, err = srv.callTool(call.ToolName, call.Arguments)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return mcp.ToolResult{IsError: true}, ctx.Err()
	case <-done:
		return result, err
	}
}

func (c *Client) ListTools(_ context.Context) ([]mcp.ToolInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var all []mcp.ToolInfo
	for _, srv := range c.servers {
		tools, err := srv.listTools()
		if err != nil {
			continue
		}
		// Apply AllowedTools filter
		cfg := c.configs[srv.config.Name]
		if len(cfg.AllowedTools) > 0 {
			filtered := tools[:0]
			for _, t := range tools {
				for _, a := range cfg.AllowedTools {
					if t.Name == a {
						filtered = append(filtered, t)
						break
					}
				}
			}
			tools = filtered
		}
		all = append(all, tools...)
	}
	return all, nil
}

func (c *Client) ListServerTools(_ context.Context, serverName string) ([]mcp.ToolInfo, error) {
	c.mu.RLock()
	srv, ok := c.servers[serverName]
	c.mu.RUnlock()
	if !ok {
		return nil, types.Newf(types.ErrCodeNotFound, "mcp server %q not connected", serverName)
	}
	return srv.listTools()
}

func (c *Client) IsConnected(serverName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.servers[serverName]
	return ok
}

func (c *Client) ConnectedServers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(c.servers))
	for n := range c.servers {
		names = append(names, n)
	}
	return names
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, srv := range c.servers {
		srv.close()
	}
	c.servers = make(map[string]*stdioServer)
	return nil
}

// Reconnect attempts to reconnect a named server (useful after crashes).
func (c *Client) Reconnect(serverName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cfg, ok := c.configs[serverName]
	if !ok {
		return types.Newf(types.ErrCodeNotFound, "mcp server %q not in config", serverName)
	}

	// Close existing if any
	if old, exists := c.servers[serverName]; exists {
		old.close()
		delete(c.servers, serverName)
	}

	srv, err := newStdioServer(cfg, c.logger)
	if err != nil {
		return err
	}
	c.servers[serverName] = srv
	return nil
}

// HealthCheck pings all connected servers via a tools/list call.
// Returns a map of server name → error (nil = healthy).
func (c *Client) HealthCheck(ctx context.Context) map[string]error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]error, len(c.servers))
	for name, srv := range c.servers {
		ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := srv.listTools()
		cancel()
		results[name] = err
		_ = ctx2
	}
	return results
}
