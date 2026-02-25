package subagent

import (
	"time"
)

// SubagentStatus represents the lifecycle state of a subagent session.
type SubagentStatus string

const (
	StatusPending   SubagentStatus = "pending"
	StatusRunning   SubagentStatus = "running"
	StatusPaused    SubagentStatus = "paused"
	StatusCompleted SubagentStatus = "completed"
	StatusFailed    SubagentStatus = "failed"
	StatusCancelled SubagentStatus = "cancelled"
	StatusRetrying  SubagentStatus = "retrying"
)

// EventType classifies what kind of event occurred during a subagent session.
type EventType string

const (
	EventLog      EventType = "log"
	EventToolCall EventType = "tool_call"
	EventResult   EventType = "result"
	EventError    EventType = "error"
	EventStatus   EventType = "status_change"
	EventRetry    EventType = "retry"
	EventPaused   EventType = "paused"
	EventResumed  EventType = "resumed"
)

// PauseReason describes why a subagent paused.
type PauseReason string

const (
	PauseToolApproval PauseReason = "tool_approval_required"
	PauseInputNeeded  PauseReason = "input_required"
)

// RetryPolicy defines how failed subagents should be retried.
type RetryPolicy struct {
	MaxRetries int `json:"max_retries"` // Maximum number of retry attempts (default: 3)
	DelayMs    int `json:"delay_ms"`    // Delay between retries in milliseconds (default: 1000)
}

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		DelayMs:    1000,
	}
}

// PendingToolCall represents a tool call awaiting approval.
type PendingToolCall struct {
	ID   string                 `json:"id"`
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// PauseInfo contains details about why a session is paused.
type PauseInfo struct {
	Reason           PauseReason       `json:"reason"`
	Message          string            `json:"message,omitempty"`
	PendingToolCalls []PendingToolCall `json:"pending_tool_calls,omitempty"`
	RawOutput        string            `json:"raw_output,omitempty"`
}

// ResumeDecision carries the master agent's decision for a paused subagent.
type ResumeDecision struct {
	Approve    []string `json:"approve,omitempty"` // Tool call IDs to approve
	Reject     []string `json:"reject,omitempty"`  // Tool call IDs to reject
	ApproveAll bool     `json:"approve_all,omitempty"`
	RejectAll  bool     `json:"reject_all,omitempty"`
	Input      string   `json:"input,omitempty"` // Follow-up text input
}

// SessionConfig defines the parameters for creating a new subagent session.
// Based on AOrchestra 4-tuple: (Instruction, Context, Tools, Model)
type SessionConfig struct {
	// AOrchestra 4-tuple
	Description  string   `json:"description"`             // Short (3-5 word) task description
	Instructions string   `json:"instructions"`            // What the subagent should do (the "I")
	Context      string   `json:"context,omitempty"`       // Curated context from previous work (the "C")
	AllowedTools []string `json:"allowed_tools,omitempty"` // Tools to grant — least-privilege (the "T")
	Model        string   `json:"model,omitempty"`         // Model override (the "M")

	// Execution parameters
	MaxSteps       int         `json:"max_steps,omitempty"`       // Max agent loop iterations (default: 30)
	TimeoutSeconds int         `json:"timeout_seconds,omitempty"` // Max wall-clock time (0 = no timeout)
	Sandbox        bool        `json:"sandbox,omitempty"`         // Run tools in sandbox
	Provider       string      `json:"provider,omitempty"`        // LLM provider override
	Retry          RetryPolicy `json:"retry,omitempty"`           // Retry policy for failed sessions

	// Approval
	PauseOnApproval bool `json:"pause_on_approval,omitempty"` // Pause before executing tools (for non-sandbox)
}

// ApplyDefaults fills in zero-valued fields with sensible defaults.
// Call this once when a session is created — eliminates default logic duplication
// across tracker, subagent tool, and HTTP server.
func (c *SessionConfig) ApplyDefaults() {
	if c.Retry.MaxRetries == 0 {
		c.Retry = DefaultRetryPolicy()
	}
	if c.MaxSteps == 0 {
		c.MaxSteps = 30
	}
	if !c.Sandbox && !c.PauseOnApproval {
		c.PauseOnApproval = true
	}
}

// RunState represents the lifecycle of a single run within a session.
// A session can have multiple runs (e.g., after a pause/resume cycle).
type RunState string

const (
	RunStateIdle     RunState = "idle"
	RunStateStarting RunState = "starting"
	RunStateRunning  RunState = "running"
	RunStateFailed   RunState = "failed"
)

// Subagent represents a spawned subagent instance.
type Subagent struct {
	ID          string         `json:"id"`
	ParentID    string         `json:"parent_id,omitempty"`   // ID of the master session
	OriginalID  string         `json:"original_id,omitempty"` // First attempt ID (for retries)
	SessionID   string         `json:"session_id"`
	RunID       string         `json:"run_id,omitempty"` // Current run ID (scoped within session)
	Config      SessionConfig  `json:"config"`
	Status      SubagentStatus `json:"status"`
	RunState    RunState       `json:"run_state"`
	Result      string         `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	RetryCount  int            `json:"retry_count"`
	PauseInfo   *PauseInfo     `json:"pause_info,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// SubagentEvent represents a single event emitted during a subagent session.
type SubagentEvent struct {
	SessionID string    `json:"session_id"`
	RunID     string    `json:"run_id,omitempty"`
	Type      EventType `json:"type"`
	Message   string    `json:"message,omitempty"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
