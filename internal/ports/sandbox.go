package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
)

// SandboxPort defines the interface for sandboxed tool execution.
// Implementations may use Docker containers, VMs, or other isolation mechanisms.
type SandboxPort interface {
	// Execute runs a tool inside a sandboxed environment.
	Execute(ctx context.Context, toolName string, args map[string]interface{}) (domain.Result, error)

	// IsAvailable reports whether the sandbox backend is operational.
	IsAvailable() bool

	// Close gracefully shuts down the sandbox environment.
	Close() error
}
