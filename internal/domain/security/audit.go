package security

import "time"

// ──────────────────────────────────
// Full Session Audit Log Domain Types
// ──────────────────────────────────

// AuditAction classifies the type of auditable event.
type AuditAction string

const (
	AuditToolExecute  AuditAction = "tool.execute"
	AuditToolResult   AuditAction = "tool.result"
	AuditFileEdit     AuditAction = "file.edit"
	AuditFileBackup   AuditAction = "file.backup"
	AuditCommand      AuditAction = "command.run"
	AuditLLMRequest   AuditAction = "llm.request"
	AuditLLMResponse  AuditAction = "llm.response"
	AuditNetworkReq   AuditAction = "network.request"
	AuditNetworkBlock AuditAction = "network.blocked"
	AuditSecretScrub  AuditAction = "secret.scrubbed"
	AuditSessionStart AuditAction = "session.start"
	AuditSessionEnd   AuditAction = "session.end"
)

// AuditEntry is a single immutable log record.
// Every action, file edit, and command is logged with full context.
type AuditEntry struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	Action    AuditAction            `json:"action"`
	Actor     string                 `json:"actor"`            // tool name / user / system
	Target    string                 `json:"target,omitempty"` // file path, URL, etc.
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	ParentID  string                 `json:"parent_id,omitempty"` // links request → response
}

// SessionSnapshot captures a point-in-time backup of all modified files.
// Stored locally — enables instant rollback of any session.
type SessionSnapshot struct {
	SessionID string            `json:"session_id"`
	Files     map[string][]byte `json:"-"` // path → original content (not serialized inline)
	FileList  []string          `json:"files"`
	CreatedAt time.Time         `json:"created_at"`
}
