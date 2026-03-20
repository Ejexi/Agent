package domain

import "time"

// HookEventType identifies which lifecycle point a hook fires at.
type HookEventType string

const (
	// HookBeforeTool fires before any tool executes. Can block execution.
	HookBeforeTool HookEventType = "BeforeTool"
	// HookAfterTool fires after a tool completes. Advisory — cannot block.
	HookAfterTool HookEventType = "AfterTool"
	// HookBeforeScan fires before MasterAgent starts a scan. Can block.
	HookBeforeScan HookEventType = "BeforeScan"
	// HookAfterScan fires after all subagents finish. Advisory.
	HookAfterScan HookEventType = "AfterScan"
	// HookSessionStart fires when a new session begins. Advisory.
	HookSessionStart HookEventType = "SessionStart"
	// HookSessionEnd fires when a session ends. Advisory.
	HookSessionEnd HookEventType = "SessionEnd"
)

// HookDecision is the value a hook script returns in its JSON output.
type HookDecision string

const (
	HookAllow HookDecision = "allow"
	HookDeny  HookDecision = "deny"
)

// HookInput is serialised as JSON and written to the hook script's stdin.
// Scripts must treat unknown fields as forward-compatible additions.
type HookInput struct {
	Event      HookEventType          `json:"event"`
	ToolName   string                 `json:"tool_name,omitempty"`
	ToolArgs   map[string]interface{} `json:"tool_args,omitempty"`
	Result     *HookToolResult        `json:"result,omitempty"` // only for AfterTool
	ScanTarget string                 `json:"scan_target,omitempty"`
	Findings   int                    `json:"findings,omitempty"` // only for AfterScan
	SessionID  string                 `json:"session_id,omitempty"`
	ProjectDir string                 `json:"project_dir,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// HookToolResult carries the outcome of a tool execution into AfterTool hooks.
type HookToolResult struct {
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// HookOutput is parsed from the hook script's stdout (must be valid JSON).
// An empty object {} is a valid "allow" response.
type HookOutput struct {
	// Decision controls whether to proceed. Absent = allow.
	Decision HookDecision `json:"decision,omitempty"`
	// Reason is shown to the user when the hook blocks.
	Reason string `json:"reason,omitempty"`
	// SystemMessage is injected into the LLM conversation as a system note.
	SystemMessage string `json:"system_message,omitempty"`
}

// HookConfig describes a single registered hook, from config.toml or directory discovery.
type HookConfig struct {
	// Name is a human-readable identifier used in logs and /hooks commands.
	Name string `toml:"name" json:"name"`
	// Matcher is a regular expression matched against the tool name.
	// Empty string matches all tools. Ignored for scan/session hooks.
	Matcher string `toml:"matcher" json:"matcher,omitempty"`
	// Command is the shell command to execute. May contain $ENV_VAR references.
	Command string `toml:"command" json:"command"`
	// TimeoutMs is the maximum execution time in milliseconds. Default: 30000.
	TimeoutMs int `toml:"timeout" json:"timeout,omitempty"`
	// Enabled controls whether the hook is active. Default: true.
	Enabled bool `toml:"enabled" json:"enabled"`
}
