package sandbox

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/shared/types"
)

// StubSandbox is a no-op implementation of the SandboxPort interface.
// It reports as unavailable and returns errors if called.
// Replace with a real Docker/Warden implementation when ready.
type StubSandbox struct{}

var _ ports.SandboxPort = (*StubSandbox)(nil)

// NewStubSandbox creates a new stub sandbox adapter.
func NewStubSandbox() *StubSandbox {
	return &StubSandbox{}
}

func (s *StubSandbox) Execute(_ context.Context, toolName string, _ map[string]interface{}) (domain.Result, error) {
	return domain.Result{
		Success: false,
		Error:   "sandbox execution is not available (stub adapter)",
	}, types.Newf(types.ErrCodeInternal, "sandbox not implemented: tool %s", toolName)
}

func (s *StubSandbox) IsAvailable() bool {
	return false
}

func (s *StubSandbox) Close() error {
	return nil
}
