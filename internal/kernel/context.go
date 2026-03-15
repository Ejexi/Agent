package kernel

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain/security"
)

// ExecutionContext wraps the standard context to include identity and capability grants.
type ExecutionContext struct {
	context.Context
	PrincipalID string
	GrantedCaps []security.Capability
}

// NewExecutionContext wraps an existing context with specific capabilities.
func NewExecutionContext(ctx context.Context, principalID string, caps []security.Capability) *ExecutionContext {
	return &ExecutionContext{
		Context:     ctx,
		PrincipalID: principalID,
		GrantedCaps: caps,
	}
}

// HasCapabilities checks whether the context has all the requested capabilities.
func (c *ExecutionContext) HasCapabilities(required []security.Capability) bool {
	if len(required) == 0 {
		return true // No capabilities required
	}

	grantedMap := make(map[security.Capability]bool, len(c.GrantedCaps))
	for _, cap := range c.GrantedCaps {
		grantedMap[cap] = true
	}

	for _, req := range required {
		if !grantedMap[req] {
			return false
		}
	}
	return true
}
