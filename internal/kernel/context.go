package kernel

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain/security"
)

// ExecutionContext wraps the standard context to include identity and capability grants.
type ExecutionContext struct {
	context.Context
	SessionID   string
	PrincipalID string
	GrantedCaps []security.Capability
	OnEvent     func(any) // Generic callback to avoid circular deps
}

// NewExecutionContext wraps an existing context with specific capabilities.
func NewExecutionContext(ctx context.Context, sessionID string, principalID string, caps []security.Capability) *ExecutionContext {
	return &ExecutionContext{
		Context:     ctx,
		SessionID:   sessionID,
		PrincipalID: principalID,
		GrantedCaps: caps,
	}
}

// WithEventCallback returns a copy of the context with the given event callback.
func (c *ExecutionContext) WithEventCallback(cb func(any)) *ExecutionContext {
	return &ExecutionContext{
		Context:     c.Context,
		SessionID:   c.SessionID,
		PrincipalID: c.PrincipalID,
		GrantedCaps: c.GrantedCaps,
		OnEvent:     cb,
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

// Emit sends an event to the internal callback if set.
func (c *ExecutionContext) Emit(event any) {
	if c.OnEvent != nil {
		c.OnEvent(event)
	}
}
