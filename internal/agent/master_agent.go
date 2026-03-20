package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	llm_domain "github.com/SecDuckOps/shared/llm/domain"
	shared_ports "github.com/SecDuckOps/shared/ports"
	scanner_domain "github.com/SecDuckOps/shared/scanner/domain"
	scanner_ports "github.com/SecDuckOps/shared/scanner/ports"
	shared_events "github.com/SecDuckOps/shared/events"
	"github.com/SecDuckOps/shared/types"
	"github.com/SecDuckOps/agent/internal/agent/subagents"
	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
)

const defaultScanTimeout = 30 * time.Minute

// MasterAgent orchestrates parallel scan subagents via the AOrchestra pattern.
type MasterAgent struct {
	scannerSvc   scanner_ports.ScannerServicePort
	llm          llm_domain.LLM
	orchestrator *Orchestrator
	hooks        ports.HookRunnerPort
	eventBus     ports.EventBusPort // nil = no progress events
	logger       shared_ports.Logger
}

// NewMasterAgent creates a MasterAgent.
func NewMasterAgent(
	scannerSvc scanner_ports.ScannerServicePort,
	llm llm_domain.LLM,
	logger shared_ports.Logger,
	hooks ports.HookRunnerPort,
	eventBus ports.EventBusPort,
) *MasterAgent {
	return &MasterAgent{
		scannerSvc:   scannerSvc,
		llm:          llm,
		orchestrator: NewOrchestrator(llm),
		hooks:        hooks,
		eventBus:     eventBus,
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

	// ── BeforeScan hook ───────────────────────────────────────────────────────
	if m.hooks != nil {
		hookInput := agent_domain.HookInput{
			Event:      agent_domain.HookBeforeScan,
			ScanTarget: target,
			SessionID:  req.SessionID,
			ProjectDir: target,
			Timestamp:  time.Now().UTC(),
		}
		out, hookErr := m.hooks.RunBeforeScan(ctx, hookInput)
		if hookErr != nil {
			reason := "BeforeScan hook blocked scan"
			if out != nil && out.Reason != "" {
				reason = out.Reason
			}
			return nil, types.Newf(types.ErrCodeInternal, "%s", reason)
		}
	}

	// ── Phase 1: Understand the project ──────────────────────────────────────
	signals := subagents.AnalyzeProject(target)
	m.logf("project signals: languages=%v iac=%v docker=%v files=%d",
		signals.Languages, signals.HasIaC, signals.HasDocker, signals.FileCount)

	// Emit scan started event for TUI streaming indicator
	if m.eventBus != nil {
		_ = m.eventBus.Publish(ctx, string(shared_events.ScanStarted), shared_events.Event{
			Type:   shared_events.ScanStarted,
			Source: "master_agent",
		})
	}

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

	result := m.aggregateResults(tracker, target, plan)

	// Emit scan completed event
	if m.eventBus != nil {
		_ = m.eventBus.Publish(ctx, string(shared_events.ScanCompleted), shared_events.Event{
			Type:   shared_events.ScanCompleted,
			Source: "master_agent",
		})
	}

	// ── AfterScan hook (advisory) ─────────────────────────────────────────────
	if m.hooks != nil {
		hookInput := agent_domain.HookInput{
			Event:      agent_domain.HookAfterScan,
			ScanTarget: target,
			Findings:   len(result.AllFindings),
			SessionID:  req.SessionID,
			ProjectDir: target,
			Timestamp:  time.Now().UTC(),
		}
		m.hooks.RunAfterScan(ctx, hookInput) //nolint:errcheck — advisory
	}

	return result, nil
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

	var allFindings []scanner_domain.Finding
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
		ByCategory: make(map[ScanCategory][]scanner_domain.Finding),
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
			case scanner_domain.SeverityCritical:
				result.Summary.Critical++
			case scanner_domain.SeverityHigh:
				result.Summary.High++
			case scanner_domain.SeverityMedium:
				result.Summary.Medium++
			case scanner_domain.SeverityLow:
				result.Summary.Low++
			case scanner_domain.SeverityInfo:
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
func filterBySeverity(findings []scanner_domain.Finding, minSeverity string) []scanner_domain.Finding {
	if minSeverity == "" {
		return findings
	}
	order := map[scanner_domain.Severity]int{
		scanner_domain.SeverityInfo: 0, scanner_domain.SeverityLow: 1,
		scanner_domain.SeverityMedium: 2, scanner_domain.SeverityHigh: 3,
		scanner_domain.SeverityCritical: 4,
	}
	minOrder, ok := order[scanner_domain.Severity(minSeverity)]
	if !ok {
		return findings
	}
	result := make([]scanner_domain.Finding, 0, len(findings))
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
