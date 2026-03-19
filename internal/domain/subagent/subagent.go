package subagent

import (
	"time"

	"github.com/SecDuckOps/shared/events"
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

// EventType is an alias to the canonical shared type.
// All new code should use events.SubagentEventType directly.
type EventType = events.SubagentEventType

// Event constants — aliases to shared/events for backward compatibility.
const (
	EventLog      = events.SubagentEventLog
	EventToolCall = events.SubagentEventToolCall
	EventResult   = events.SubagentEventResult
	EventError    = events.SubagentEventError
	EventStatus   = events.SubagentEventStatus
	EventRetry    = events.SubagentEventRetry
	EventPaused   = events.SubagentEventPaused
	EventResumed  = events.SubagentEventResumed
	EventThought  = events.SubagentEventThought
)

// PauseReason describes why a subagent paused.
type PauseReason string

const (
	PauseToolApproval PauseReason = "tool_approval_required"
	PauseInputNeeded  PauseReason = "input_required"
)

// RetryPolicy defines how failed subagents should be retried.
type RetryPolicy struct {
	MaxRetries int `json:"max_retries"`
	DelayMs    int `json:"delay_ms"`
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
	Approve    []string `json:"approve,omitempty"`
	Reject     []string `json:"reject,omitempty"`
	ApproveAll bool     `json:"approve_all,omitempty"`
	RejectAll  bool     `json:"reject_all,omitempty"`
	Input      string   `json:"input,omitempty"`
}

// SessionConfig defines the parameters for creating a new subagent session.
type SessionConfig struct {
	Description  string   `json:"description"`
	Instructions string   `json:"instructions"`
	Context      string   `json:"context,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	Model        string   `json:"model,omitempty"`

	MaxSteps       int         `json:"max_steps,omitempty"`
	TimeoutSeconds int         `json:"timeout_seconds,omitempty"`
	Sandbox        bool        `json:"sandbox,omitempty"`
	Provider       string      `json:"provider,omitempty"`
	Retry          RetryPolicy `json:"retry,omitempty"`

	PauseOnApproval bool `json:"pause_on_approval,omitempty"`
}

// ApplyDefaults fills in zero-valued fields with sensible defaults.
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
	ParentID    string         `json:"parent_id,omitempty"`
	OriginalID  string         `json:"original_id,omitempty"`
	SessionID   string         `json:"session_id"`
	RunID       string         `json:"run_id,omitempty"`
	Config      SessionConfig  `json:"config"`
	Status      SubagentStatus `json:"status"`
	RunState    RunState       `json:"run_state"`
	Result      string         `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	RetryCount  int            `json:"retry_count"`
	PauseInfo   *PauseInfo     `json:"pause_info,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	Depth       int            `json:"depth"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// SubagentEvent is an alias to the canonical shared type.
// All new code should use events.SubagentEvent directly.
type SubagentEvent = events.SubagentEvent
