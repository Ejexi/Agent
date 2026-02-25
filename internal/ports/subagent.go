package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain/subagent"
)

// SubagentPort defines the interface for subagent session lifecycle management.
// The kernel uses this port to spawn, track, and stream events from subagent sessions.
type SubagentPort interface {
	// CreateSession initializes a new subagent session and returns its unique ID.
	CreateSession(ctx context.Context, config subagent.SessionConfig) (sessionID string, err error)

	// SendMessage sends a user message to an active session's agent loop.
	SendMessage(ctx context.Context, sessionID string, message string) error

	// GetStatus retrieves the current status of a subagent session.
	GetStatus(ctx context.Context, sessionID string) (*subagent.Subagent, error)

	// StreamEvents returns a channel that emits events from the specified session.
	// The channel is closed when the session completes or is cancelled.
	StreamEvents(ctx context.Context, sessionID string) (<-chan subagent.SubagentEvent, error)

	// CancelSession terminates a running subagent session.
	CancelSession(ctx context.Context, sessionID string) error
}
