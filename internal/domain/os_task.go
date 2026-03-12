package domain

// OSTask represents a low-level OS command requested by the Agent.
// This is distinct from a Kernel Task, which is an LLM Tool Call.
// The OSTask is what travels through the Task Engine Middleware pipeline.
type OSTask struct {
	OriginalCmd string
	Args        []string
	Cwd         string
	Env         map[string]string // Optional environment variables
}

// OSExecutionStatus represents the lifecycle state of an OS command.
type OSExecutionStatus string

const (
	StatusPending   OSExecutionStatus = "pending"
	StatusRunning   OSExecutionStatus = "running"
	StatusCompleted OSExecutionStatus = "completed"
	StatusFailed    OSExecutionStatus = "failed"
	StatusCancelled OSExecutionStatus = "cancelled"
	StatusTimedOut  OSExecutionStatus = "timed_out"
)

// OSTaskResult is the universal output format for OS commands executed via the Task Engine.
type OSTaskResult struct {
	Status   OSExecutionStatus      `json:"status"`
	Stdout   string                 `json:"stdout,omitempty"`
	Stderr   string                 `json:"stderr,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"` // For structured pipeline outputs
	Error    error                  `json:"-"`              // Go internal error handling
	ExitCode int                    `json:"exit_code"`
	
	// Metrics
	DurationMs int64 `json:"duration_ms"`
}
