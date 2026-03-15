package security

import (
	"time"
)

// ScanSpec defines the requirements for a security tool execution.
type ScanSpec struct {
	ToolName    string            `json:"tool_name"`
	Image       string            `json:"image"`
	Command     []string          `json:"command"`
	TargetPath  string            `json:"target_path"`
	Timeout     time.Duration     `json:"timeout"`
	CPUQuota    int64             `json:"cpu_quota"`
	MemoryLimit int64             `json:"memory_limit"`
	EnvVars     map[string]string `json:"env_vars"`
}

// ScanResult contains the outcome of a completed scan.
type ScanResult struct {
	RawOutput []byte        `json:"raw_output"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration"`
	Tool      string        `json:"tool"`
}
