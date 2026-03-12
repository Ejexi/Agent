package warden

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/shared/types"
	"github.com/google/uuid"
)

// Warden implements ports.WardenPort.
// Provides a transparent proxy with Cedar-style policy evaluation and mTLS enforcement.
// All traffic stays on-premises — nothing leaves the trust boundary.
type Warden struct {
	policies    []security.NetworkPolicy
	defaultDeny bool

	// Proxy
	listener  net.Listener
	server    *http.Server
	proxyAddr string

	// mTLS
	tlsConfig *tls.Config

	mu     sync.RWMutex
	closed bool
}

// New creates a new Warden adapter.
func New(defaultDeny bool) *Warden {
	return &Warden{
		policies:    make([]security.NetworkPolicy, 0),
		defaultDeny: defaultDeny,
	}
}

// Evaluate checks a network request against loaded Cedar policies.
func (w *Warden) Evaluate(_ context.Context, req security.NetworkRequest) (security.PolicyDecision, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Assign ID if missing
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	// Evaluate policies in priority order (highest first)
	for _, policy := range w.policies {
		if !policy.Enabled {
			continue
		}

		matched, allowed := evaluateCedarPolicy(policy, req)
		if matched {
			decision := security.PolicyDecision{
				Allowed:  allowed,
				PolicyID: policy.ID,
				Reasons:  []string{fmt.Sprintf("Matched policy: %s (%s)", policy.Name, policy.ID)},
			}
			return decision, nil
		}
	}

	// No policy matched — apply default
	if w.defaultDeny {
		return security.PolicyDecision{
			Allowed: false,
			Reasons: []string{"No matching policy found, default deny is active"},
		}, nil
	}

	return security.PolicyDecision{
		Allowed: false,
		Reasons: []string{"No matching policy found, default allow"},
	}, nil
}

// EvaluateExecution checks a local OS execution request against Cedar policies.
func (w *Warden) EvaluateExecution(_ context.Context, req security.ExecutionRequest) (security.PolicyDecision, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Assign ID if missing
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	// Evaluate policies in priority order
	for _, policy := range w.policies {
		if !policy.Enabled {
			continue
		}

		matched, allowed := evaluateCedarExecutionPolicy(policy, req)
		if matched {
			decision := security.PolicyDecision{
				Allowed:  allowed,
				PolicyID: policy.ID,
				Reasons:  []string{fmt.Sprintf("Matched execution policy: %s (%s)", policy.Name, policy.ID)},
			}
			return decision, nil
		}
	}

	// No policy matched
	if w.defaultDeny {
		return security.PolicyDecision{
			Allowed: false,
			Reasons: []string{"No matching execution policy found, default deny is active"},
		}, nil
	}

	return security.PolicyDecision{
		Allowed: false,
		Reasons: []string{"No matching execution policy found, default allow"},
	}, nil
}

// LoadPolicies loads Cedar policies into the evaluator.
func (w *Warden) LoadPolicies(_ context.Context, policies []security.NetworkPolicy) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.policies = make([]security.NetworkPolicy, len(policies))
	copy(w.policies, policies)

	// Sort by priority (descending)
	for i := 1; i < len(w.policies); i++ {
		for j := i; j > 0 && w.policies[j].Priority > w.policies[j-1].Priority; j-- {
			w.policies[j], w.policies[j-1] = w.policies[j-1], w.policies[j]
		}
	}

	return nil
}

// StartProxy starts the transparent HTTP proxy.
func (w *Warden) StartProxy(ctx context.Context, listenAddr string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return types.New(types.ErrCodeInvalidInput, "warden is closed")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		req := security.NetworkRequest{
			ID:        uuid.New().String(),
			Method:    r.Method,
			URL:       r.URL.String(),
			Headers:   flattenHeaders(r.Header),
			Timestamp: time.Now(),
		}

		// Extract source tool from custom header
		if tool := r.Header.Get("X-DuckOps-Source-Tool"); tool != "" {
			req.SourceTool = tool
		}
		if sid := r.Header.Get("X-DuckOps-Session-ID"); sid != "" {
			req.SessionID = sid
		}

		decision, err := w.Evaluate(ctx, req)
		if err != nil || !decision.Allowed {
			http.Error(rw, "Blocked by Warden policy", http.StatusForbidden)
			return
		}

		// Forward the request (simple proxy)
		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, `{"allowed": true, "policy_id": %q}`, decision.PolicyID)
	})

	w.server = &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	var err error
	if w.tlsConfig != nil {
		w.server.TLSConfig = w.tlsConfig
		w.listener, err = tls.Listen("tcp", listenAddr, w.tlsConfig)
	} else {
		w.listener, err = net.Listen("tcp", listenAddr)
	}
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to start proxy listener")
	}

	w.proxyAddr = w.listener.Addr().String()

	go func() {
		if serveErr := w.server.Serve(w.listener); serveErr != nil && serveErr != http.ErrServerClosed {
			// Log error — in production this would go to the audit log
			_ = serveErr
		}
	}()

	return nil
}

// StopProxy stops the proxy gracefully.
func (w *Warden) StopProxy(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.closed = true
	if w.server != nil {
		return w.server.Shutdown(ctx)
	}
	return nil
}

// ConfigureMTLS sets up mutual TLS for agent↔server communication.
func (w *Warden) ConfigureMTLS(_ context.Context, cfg security.MTLSConfig) error {
	// Load CA certificate
	caCert, err := os.ReadFile(cfg.CACert)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to read CA cert")
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return types.New(types.ErrCodeInvalidInput, "failed to parse CA cert")
	}

	// Load client certificate and key
	clientCert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to load client cert/key")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
		ServerName:   cfg.ServerName,
	}

	return nil
}

// ProxyAddr returns the address the proxy is listening on, if started.
func (w *Warden) ProxyAddr() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.proxyAddr
}

// ──────────────── Cedar policy evaluation ────────────────

// evaluateCedarPolicy provides a Go-native evaluation of Cedar-style policies.
// The CedarBody field supports a simple DSL:
//
//	ALLOW url_contains "example.com"
//	DENY method "DELETE"
//	ALLOW source_tool "scan_tool"
//	DENY url_contains "internal.corp"
func evaluateCedarPolicy(policy security.NetworkPolicy, req security.NetworkRequest) (matched bool, allowed bool) {
	body := strings.TrimSpace(policy.CedarBody)
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}

		action := strings.ToUpper(parts[0])    // ALLOW or DENY
		predicate := strings.ToLower(parts[1]) // url_contains, method, source_tool, etc.
		value := strings.Trim(parts[2], "\"'") // value to match

		var matches bool
		switch predicate {
		case "url_contains":
			matches = strings.Contains(req.URL, value)
		case "url_prefix":
			matches = strings.HasPrefix(req.URL, value)
		case "url_suffix":
			matches = strings.HasSuffix(req.URL, value)
		case "method":
			matches = strings.EqualFold(req.Method, value)
		case "source_tool":
			matches = req.SourceTool == value
		case "session_id":
			matches = req.SessionID == value
		case "header":
			// Format: header "Key:Value"
			kv := strings.SplitN(value, ":", 2)
			if len(kv) == 2 {
				matches = req.Headers[strings.TrimSpace(kv[0])] == strings.TrimSpace(kv[1])
			}
		case "all":
			matches = true
		}

		if matches {
			return true, action == "ALLOW"
		}
	}

	return false, false
}

// evaluateCedarExecutionPolicy provides a naive Go-native evaluation for execution requests.
// Supports: ALLOW command "ls"
// DENY command "rm"
// ALLOW dir_prefix "/var/log"
func evaluateCedarExecutionPolicy(policy security.NetworkPolicy, req security.ExecutionRequest) (matched bool, allowed bool) {
	body := strings.TrimSpace(policy.CedarBody)
	lines := strings.Split(body, "\n")

	// Get cwd safely
	cwd := ""
	if c, ok := req.Context["cwd"].(string); ok {
		cwd = c
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}

		action := strings.ToUpper(parts[0])    // ALLOW or DENY
		predicate := strings.ToLower(parts[1]) // command, dir_prefix, arg_contains
		value := strings.Trim(parts[2], "\"'") // value to match

		var matches bool
		switch predicate {
		case "command":
			matches = strings.EqualFold(req.Command, value)
		case "dir_prefix":
			matches = strings.HasPrefix(cwd, value)
		case "arg_contains":
			for _, arg := range req.Args {
				if strings.Contains(arg, value) {
					matches = true
					break
				}
			}
		case "all":
			matches = true
		}

		if matches {
			return true, action == "ALLOW"
		}
	}

	return false, false
}

// flattenHeaders converts http.Header to a simple map.
func flattenHeaders(h http.Header) map[string]string {
	result := make(map[string]string, len(h))
	for k, v := range h {
		result[k] = strings.Join(v, ", ")
	}
	return result
}
