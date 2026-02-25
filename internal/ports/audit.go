package ports

import (
	"context"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/security"
)

// AuditLogPort defines the interface for full session audit logging.
// Every action, file edit, and command is logged with full context.
// Changes are backed up before modification. Replay any session, roll back instantly.
type AuditLogPort interface {
	// Record writes an immutable audit entry to the log.
	Record(ctx context.Context, entry security.AuditEntry) error

	// Query returns audit entries matching the filter.
	Query(ctx context.Context, filter AuditFilter) ([]security.AuditEntry, error)

	// BackupSession creates a snapshot backup of all files modified in the session.
	BackupSession(ctx context.Context, snapshot security.SessionSnapshot) error

	// ReplaySession returns all audit entries for a session in chronological order.
	ReplaySession(ctx context.Context, sessionID string) ([]security.AuditEntry, error)

	// Close gracefully shuts down the audit log backend.
	Close() error
}

// AuditFilter provides criteria for querying audit entries.
type AuditFilter struct {
	SessionID string               `json:"session_id,omitempty"`
	Action    security.AuditAction `json:"action,omitempty"`
	Actor     string               `json:"actor,omitempty"`
	From      *time.Time           `json:"from,omitempty"`
	To        *time.Time           `json:"to,omitempty"`
	Limit     int                  `json:"limit,omitempty"`
}
