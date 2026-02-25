package security

import "time"

// ──────────────────────────────────────
// Network Sandbox (Warden) Domain Types
// ──────────────────────────────────────

// NetworkRequest represents an outbound request captured by the Warden proxy.
type NetworkRequest struct {
	ID         string            `json:"id"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers,omitempty"`
	SourceTool string            `json:"source_tool"` // which tool/MCP server originated it
	SessionID  string            `json:"session_id"`
	Timestamp  time.Time         `json:"timestamp"`
}

// PolicyDecision is the result of evaluating a Cedar policy against a request.
type PolicyDecision struct {
	Allowed  bool     `json:"allowed"`
	PolicyID string   `json:"policy_id,omitempty"` // which policy matched
	Reasons  []string `json:"reasons,omitempty"`   // explanation chain
}

// NetworkPolicy is a single Cedar-style policy with an ID.
type NetworkPolicy struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CedarBody string `json:"cedar_body"` // raw Cedar policy text
	Priority  int    `json:"priority"`
	Enabled   bool   `json:"enabled"`
}

// MTLSConfig holds certificate paths for mutual TLS.
// All paths are local — no external PKI dependencies.
type MTLSConfig struct {
	CACert     string `json:"ca_cert" toml:"ca_cert"`         // path to CA certificate
	ClientCert string `json:"client_cert" toml:"client_cert"` // path to client certificate
	ClientKey  string `json:"client_key" toml:"client_key"`   // path to client private key
	ServerName string `json:"server_name" toml:"server_name"` // expected server CN
}
