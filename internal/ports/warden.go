package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain/security"
)

// WardenPort defines the interface for the network sandbox proxy.
// All traffic goes through a transparent proxy with Cedar policy evaluation.
// Runs entirely on-premises with mTLS — no data leaves the environment.
type WardenPort interface {
	// Evaluate checks a network request against loaded Cedar policies.
	Evaluate(ctx context.Context, req security.NetworkRequest) (security.PolicyDecision, error)

	// LoadPolicies loads Cedar policies from config or file.
	LoadPolicies(ctx context.Context, policies []security.NetworkPolicy) error

	// StartProxy starts the transparent HTTP/HTTPS proxy.
	StartProxy(ctx context.Context, listenAddr string) error

	// StopProxy stops the proxy gracefully.
	StopProxy(ctx context.Context) error

	// ConfigureMTLS sets up mutual TLS for agent↔server communication.
	ConfigureMTLS(ctx context.Context, cfg security.MTLSConfig) error
}
