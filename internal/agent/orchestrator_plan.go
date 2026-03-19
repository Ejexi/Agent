package agent

// OrchestratorPlan is the structured execution plan produced by the MasterAgent's LLM
// before any scanner runs. It follows the AOrchestra pattern:
// the master reasons first, then delegates with full context.
//
// This matches the system prompt's JSON output contract exactly.
type OrchestratorPlan struct {
	SystemSummary  string           `json:"system_summary"`
	DetectedSignals []RiskSignal    `json:"detected_signals"`
	RiskOverview   string           `json:"risk_overview"`
	ExecutionPlan  StagePlan        `json:"execution_plan"`
	ExpectedInsights string         `json:"expected_insights"`
	Confidence     float64          `json:"confidence"`
}

// RiskSignal is a single detected risk indicator with weight.
type RiskSignal struct {
	Type   string  `json:"type"`   // e.g. SECURITY_SIGNALS, ARCHITECTURE_SIGNALS
	Weight float64 `json:"weight"` // 0.0 – 1.0
	Reason string  `json:"reason"`
}

// StagePlan holds the ordered execution stages.
type StagePlan struct {
	Stages []ExecutionStage `json:"stages"`
}

// ExecutionStage is one DAG node — may run in parallel with siblings.
type ExecutionStage struct {
	ID        string          `json:"id"`
	Parallel  bool            `json:"parallel"`
	Objective string          `json:"objective"`
	Tasks     []PlannedTask   `json:"tasks"`
}

// PlannedTask is one unit of work inside a stage.
type PlannedTask struct {
	TaskType  string `json:"task_type"`  // "tool" | "reasoning" | "mapping" | "correlation"
	Target    string `json:"target"`     // e.g. "sast", "secrets", "repo"
	Depth     int    `json:"depth"`      // 1 = light, 2 = standard, 3 = deep
	Priority  int    `json:"priority"`   // 1 = highest
	Rationale string `json:"rationale"`
}

// SubagentContext is the 4-tuple passed to each subagent per AOrchestra:
// (Instruction, Context, Tools, Model)
type SubagentContext struct {
	Category    ScanCategory
	Instruction string   // what to focus on — from the orchestrator plan
	Context     string   // system summary + risk signals — shared understanding
	Depth       int      // 1 | 2 | 3 — how deep to scan
	Priority    int      // 1 = highest
}
