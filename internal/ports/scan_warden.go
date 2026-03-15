package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain/security"
)

// Warden defines the interface for the secure container runtime.
type Warden interface {
	Run(ctx context.Context, spec security.ScanSpec) (security.ScanResult, error)
}
