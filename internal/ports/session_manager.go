package ports

import (
	"github.com/SecDuckOps/agent/internal/domain/subagent"
)

// SessionManager defines the interface for managing subagent sessions.
// The Tracker implements this; the HTTP server and tools consume it.
// This decouples the server adapter from the concrete Tracker implementation.
type SessionManager interface {
	SpawnSubagent(parentID string, config subagent.SessionConfig) (string, error)
	GetSession(sessionID string) (SessionView, error)
	ListSessions() []subagent.Subagent
	CancelSession(sessionID string) error
	ResumeSession(sessionID string, decision subagent.ResumeDecision) error

	// Streaming
	StreamEvents(sessionID string) (subID uint64, events <-chan IndexedEvent, err error)
	UnsubscribeEvents(sessionID string, subID uint64)
	ReplayEvents(sessionID string, sinceSeqID uint64) ([]IndexedEvent, error)
}

// SessionView provides read access to a session's state.
// Avoids exposing the concrete SubagentSession struct from the subagent package.
type SessionView struct {
	Subagent subagent.Subagent
}

// IndexedEvent wraps a SubagentEvent with a monotonic sequence ID.
// Mirrors the subagent.IndexedEvent type at the port boundary.
type IndexedEvent struct {
	SeqID uint64                 `json:"seq_id"`
	Event subagent.SubagentEvent `json:"event"`
}
