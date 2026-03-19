package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	llm_domain "github.com/SecDuckOps/shared/llm/domain"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/scanner/domain"
	scanner_ports "github.com/SecDuckOps/shared/scanner/ports"
	"github.com/SecDuckOps/shared/types"
	"github.com/SecDuckOps/agent/internal/agent/subagents"
)

const defaultScanTimeout = 30 * time.Minute

// MasterAgent orchestrates parallel scan subagents via the AOrchestra pattern.
//
// Transport is fully abstracted behind ScannerServicePort — previously Docker,
// now MCP. The orchestration logic (AOrchestra 3-phase) is unchanged.
type MasterAgent struct {
	scannerSvc   scanner_ports.ScannerServicePort
	llm          llm_domain.LLM
	orchestrator *Orchestrator
	logger       shared_ports.Logger
}

// NewMasterAgent creates a MasterAgent.
// scannerSvc nil → scans fail gracefully with a clear error.
func NewMasterAgent(scannerSvc scanner_ports.ScannerServicePort, llm llm_domain.LLM, logger shared_ports.Logger) *MasterAgent {
	return &MasterAgent{
		scannerSvc:   scannerSvc,
		llm:          llm,
		orchestrator: NewOrchestrator(llm),
		logger:       logger,
	}
}

// ScannerSvc exposes the scanner service for health checks.
func (m *MasterAgent) ScannerSvc() scanner_ports.ScannerServicePort {
	return m.scannerSvc
}

// HandleScanRequest is the main entry point.
func (m *MasterAgent) HandleScanRequest(ctx context.Context, req ScanRequest) (*AggregatedResult, error) {
	if m.scannerSvc == nil {
		return nil, types.New(types.ErrCodeInternal, "scanner service unavailable: Docker is not running")
	}

	// Resolve target to absolute path
	target, err := filepath.Abs(req.TargetPath)
	if err != nil {
		target = req.TargetPath
	}
	req.TargetPath = target

	// ── Phase 1: Understand the project ──────────────────────────────────────
	signals := subagents.AnalyzeProject(target)
	m.logf("project signals: languages=%v iac=%v docker=%v files=%d",
		signals.Languages, signals.HasIaC, signals.HasDocker, signals.FileCount)

	// ── Phase 2: Plan execution (LLM produces structured DAG) ────────────────
	plan, err := m.orchestrator.Plan(ctx, signals, req)
	if err != nil {
		m.logf("orchestrator planning failed (using static fallback): %v", err)
	}
	m.logf("plan ready: confidence=%.2f signals=%d stages=%d",
		plan.Confidence, len(plan.DetectedSignals), len(plan.ExecutionPlan.Stages))

	// Extract SubagentContext 4-tuples from the plan
	contexts := m.orchestrator.ExtractSubagentContexts(plan)
	if len(contexts) == 0 {
		return nil, types.New(types.ErrCodeInternal, "orchestrator produced no executable tasks")
	}

	// Sort by priority (1 = most critical first, but all run in parallel anyway)
	sort.Slice(contexts, func(i, j int) bool {
		return contexts[i].Priority < contexts[j].Priority
	})

	m.logf("spawning %d subagents: %v", len(contexts), categoryNames(contexts))

	// ── Phase 3: Execute subagents in parallel ────────────────────────────────
	tracker := NewScanTaskTracker()
	taskIDs := make([]TaskID, 0, len(contexts))

	for _, sc := range contexts {
		task := tracker.Register(sc.Category, target)
		taskIDs = append(taskIDs, task.ID)
		go m.runSubagent(ctx, tracker, task, sc, req)
	}

	// Wait for all to reach terminal state
	if err := tracker.WaitForAll(ctx, taskIDs, defaultScanTimeout); err != nil {
		m.logf("wait ended: %v — returning partial results", err)
	}

	return m.aggregateResults(tracker, target, plan), nil
}

// runSubagent executes one subagent with its 4-tuple context.
// Runs in a goroutine. Updates tracker on completion.
func (m *MasterAgent) runSubagent(
	ctx context.Context,
	tracker *ScanTaskTracker,
	task *ScanTask,
	sc SubagentContext,
	req ScanRequest,
) {
	tracker.SetRunning(task.ID)
	m.logf("subagent %s started (depth=%d priority=%d)", sc.Category, sc.Depth, sc.Priority)

	allowed, ok := subagentScanners[sc.Category]
	if !ok {
		tracker.SetFailed(task.ID, fmt.Sprintf("unknown category: %s", sc.Category))
		return
	}

	// Determine scanners based on depth, then filter to only available ones
	scanners := selectByDepth(allowed, sc.Depth)
	if m.scannerSvc != nil {
		var available []string
		for _, s := range scanners {
			if m.scannerSvc.HasScanner(s) {
				available = append(available, s)
			}
		}
		if len(available) > 0 {
			scanners = available
		}
	}

	// Run all scanners in parallel via RunScanBatch
	batchResults := m.scannerSvc.RunScanBatch(ctx, req.TargetPath, scanners)

	var allFindings []domain.Finding
	for _, br := range batchResults {
		if br.Err != nil {
			m.logf("scanner %s failed (non-fatal): %v", br.ScannerName, br.Err)
			continue
		}
		findings := filterBySeverity(br.Result.Findings, req.MinSeverity)
		m.logf("scanner %s: %d findings", br.ScannerName, len(findings))
		allFindings = append(allFindings, findings...)
	}

	task.OrchestratorInstruction = sc.Instruction
	task.OrchestratorContext = sc.Context

	tracker.SetCompleted(task.ID, allFindings)
	m.logf("subagent %s done: %d findings", sc.Category, len(allFindings))
}

// aggregateResults merges all completed tasks into one AggregatedResult.
func (m *MasterAgent) aggregateResults(tracker *ScanTaskTracker, targetPath string, plan *OrchestratorPlan) *AggregatedResult {
	result := &AggregatedResult{
		TargetPath: targetPath,
		ByCategory: make(map[ScanCategory][]domain.Finding),
		Tasks:      tracker.ListAll(),
		Plan:       plan,
	}

	for _, task := range result.Tasks {
		if task.Status != ScanTaskCompleted {
			continue
		}
		result.ByCategory[task.Category] = task.Findings
		for _, f := range task.Findings {
			result.AllFindings = append(result.AllFindings, f)
			switch f.Severity {
			case domain.SeverityCritical:
				result.Summary.Critical++
			case domain.SeverityHigh:
				result.Summary.High++
			case domain.SeverityMedium:
				result.Summary.Medium++
			case domain.SeverityLow:
				result.Summary.Low++
			case domain.SeverityInfo:
				result.Summary.Info++
			}
		}
	}

	result.Summary.Total = len(result.AllFindings)
	return result
}

// selectByDepth picks which scanners to run based on depth level.
// depth=1 → primary scanner only
// depth=2 → first two (standard)
// depth=3 → all (deep audit)
func selectByDepth(scanners []string, depth int) []string {
	switch {
	case depth <= 1:
		if len(scanners) > 0 {
			return scanners[:1]
		}
	case depth == 2:
		if len(scanners) > 2 {
			return scanners[:2]
		}
	}
	return scanners // depth=3 or fallback: run all
}

// filterBySeverity removes findings below minSeverity.
func filterBySeverity(findings []domain.Finding, minSeverity string) []domain.Finding {
	if minSeverity == "" {
		return findings
	}
	order := map[domain.Severity]int{
		domain.SeverityInfo: 0, domain.SeverityLow: 1,
		domain.SeverityMedium: 2, domain.SeverityHigh: 3,
		domain.SeverityCritical: 4,
	}
	minOrder, ok := order[domain.Severity(minSeverity)]
	if !ok {
		return findings
	}
	result := make([]domain.Finding, 0, len(findings))
	for _, f := range findings {
		if order[f.Severity] >= minOrder {
			result = append(result, f)
		}
	}
	return result
}

func categoryNames(contexts []SubagentContext) []string {
	names := make([]string, len(contexts))
	for i, c := range contexts {
		names[i] = string(c.Category)
	}
	return names
}

func (m *MasterAgent) logf(format string, args ...interface{}) {
	if m.logger == nil {
		return
	}
	m.logger.Info(context.Background(),
		fmt.Sprintf("[MasterAgent] "+format, args...),
	)
}

func (m *MasterAgent) logErr(err error, msg string) {
	if m.logger == nil {
		return
	}
	m.logger.ErrorErr(context.Background(), err, "[MasterAgent] "+msg,
		shared_ports.Field{Key: "error_code", Value: types.FromError(err).Code},
	)
}
