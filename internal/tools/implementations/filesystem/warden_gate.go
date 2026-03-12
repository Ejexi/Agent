package filesystem

import (
	"context"
	"fmt"

	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/shared/types"
)

// WardenGate wraps filesystem operations with Warden Cedar policy checks.
// Every FS operation passes through the Warden before execution.
type WardenGate struct {
	warden ports.WardenPort
}

// NewWardenGate creates a new WardenGate.
// If warden is nil, all operations are allowed (Stand Duck without explicit policies).
func NewWardenGate(warden ports.WardenPort) *WardenGate {
	return &WardenGate{warden: warden}
}

// CheckRead evaluates whether a read operation is permitted.
func (g *WardenGate) CheckRead(ctx context.Context, path string) error {
	return g.evaluate(ctx, "GET", path, "filesystem", "read")
}

// CheckWrite evaluates whether a write operation is permitted.
func (g *WardenGate) CheckWrite(ctx context.Context, path string) error {
	return g.evaluate(ctx, "PUT", path, "filesystem", "write")
}

// CheckDelete evaluates whether a delete operation is permitted.
func (g *WardenGate) CheckDelete(ctx context.Context, path string) error {
	return g.evaluate(ctx, "DELETE", path, "filesystem", "delete")
}

// CheckList evaluates whether a list/directory operation is permitted.
func (g *WardenGate) CheckList(ctx context.Context, path string) error {
	return g.evaluate(ctx, "GET", path, "filesystem", "list")
}

// evaluate runs the Warden policy check.
func (g *WardenGate) evaluate(ctx context.Context, method, path, tool, operation string) error {
	// If no Warden is configured, allow by default
	if g.warden == nil {
		return nil
	}

	req := security.NetworkRequest{
		Method:     method,
		URL:        fmt.Sprintf("file://%s", path),
		SourceTool: tool,
		Headers: map[string]string{
			"X-Operation": operation,
		},
	}

	decision, err := g.warden.Evaluate(ctx, req)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "warden policy evaluation failed")
	}

	if !decision.Allowed {
		reason := "policy denied"
		if len(decision.Reasons) > 0 {
			reason = decision.Reasons[0]
		}
		return types.Newf(types.ErrCodePermissionDenied,
			"filesystem %s on %q blocked by Warden: %s (policy: %s)",
			operation, path, reason, decision.PolicyID)
	}

	return nil
}
