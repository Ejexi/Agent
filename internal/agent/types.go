package agent

import (
	"github.com/SecDuckOps/shared/scanner/domain"
)

// ScanRequest is the structured input to the MasterAgent.
// It comes from ParseIntent (CLI) or directly from the HTTP API.
type ScanRequest struct {
	TargetPath  string // absolute or relative path to scan
	Categories  []string // empty = all enabled. e.g. ["sast","secrets"]
	MinSeverity string   // "" | "high" | "critical"
	OutputFmt   string   // "text" | "json"

	// per-category flags (set from Categories or config defaults)
	RunSAST    bool
	RunSCA     bool
	RunSecrets bool
	RunIaC     bool
	RunDeps    bool
}

// TaskPlan describes which scan subagents to spawn and how.
type TaskPlan struct {
	Subagents []ScanCategory // ordered list — all run in parallel
	Parallel  bool
}

// ScanCategory identifies a scan subagent by category.
type ScanCategory string

const (
	CategorySAST    ScanCategory = "sast"
	CategorySCA     ScanCategory = "sca"
	CategorySecrets ScanCategory = "secrets"
	CategoryIaC     ScanCategory = "iac"
	CategoryDeps    ScanCategory = "deps"
)

// subagentScanners maps each category to its allowed scanner names.
// This is the single source of truth for least-privilege enforcement.
var subagentScanners = map[ScanCategory][]string{
	CategorySAST:    {"semgrep", "gosec", "bandit", "njsscan", "brakeman"},
	CategorySCA:     {"trivy", "grype", "osvscanner", "dependencycheck"},
	CategorySecrets: {"gitleaks", "trufflehog", "detectsecrets"},
	CategoryIaC:     {"checkov", "tfsec", "kics", "terrascan", "tflint"},
	CategoryDeps:    {"osvscanner", "dependencycheck"},
}

// TaskID is a short human-readable scan task identifier.
type TaskID string

// ScanTaskStatus is the lifecycle state of one scan subagent task.
type ScanTaskStatus string

const (
	ScanTaskStarting  ScanTaskStatus = "starting"
	ScanTaskRunning   ScanTaskStatus = "running"
	ScanTaskCompleted ScanTaskStatus = "completed"
	ScanTaskFailed    ScanTaskStatus = "failed"
	ScanTaskCancelled ScanTaskStatus = "cancelled"
	ScanTaskTimedOut  ScanTaskStatus = "timed_out"
)

// ScanTask tracks one category's execution state.
type ScanTask struct {
	ID         TaskID
	Category   ScanCategory
	TargetPath string
	Status     ScanTaskStatus
	Findings   []domain.Finding
	Error      string
	// Set by MasterAgent from the orchestrator plan (AOrchestra 4-tuple)
	OrchestratorInstruction string
	OrchestratorContext     string
}

// AggregatedResult is what MasterAgent returns after all subagents finish.
type AggregatedResult struct {
	TargetPath  string
	ByCategory  map[ScanCategory][]domain.Finding
	AllFindings []domain.Finding
	Summary     FindingSummary
	Tasks       []*ScanTask
	// Plan is the orchestrator's structured execution plan (for reporting/debugging)
	Plan *OrchestratorPlan
}

// FindingSummary counts findings by severity.
type FindingSummary struct {
	Total    int
	Critical int
	High     int
	Medium   int
	Low      int
	Info     int
}
