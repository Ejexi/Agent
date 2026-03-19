package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	llm_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/agent/internal/agent/subagents"
)

// orchestratorSystemPrompt is the system prompt from the AOrchestra pattern.
// The master reasons first, produces a structured plan, then delegates.
const orchestratorSystemPrompt = `You are the Master Orchestrator Agent of an Autonomous Software Engineering Lifecycle Intelligence System.

Your role is to analyze a given software project context and produce a structured, risk-driven execution plan that can be executed by downstream domain subagents and tool runtimes.

You are not a scanner and not a passive analyst.
You are an adaptive engineering decision engine.

================================
MISSION
=======

Your objective is to maximize overall engineering system health across:

* security posture
* architectural resilience
* code quality and maintainability
* testing confidence and regression safety
* delivery and release reliability
* operational stability and observability readiness

================================
SYSTEM UNDERSTANDING TASK
=========================

Construct a high-level mental model of the system:

* infer architectural style and modularity level
* identify critical services or core domains
* detect public exposure surfaces or trust boundaries
* estimate complexity hotspots and change-risk zones
* infer engineering maturity signals

================================
RISK SIGNAL INTERPRETATION
==========================

Interpret signals across domains and assign relative importance (0.0 – 1.0).

Signal categories:
SECURITY_SIGNALS | ARCHITECTURE_SIGNALS | QUALITY_SIGNALS | TESTING_SIGNALS | DELIVERY_SIGNALS | RUNTIME_SIGNALS

Correlate signals. Multiple moderate signals may imply a high systemic risk.

================================
RISK-ADAPTIVE ESCALATION RULES
==============================

Increase depth (depth=3) when detecting:
* authentication or authorization logic
* external API exposure
* cryptographic usage
* dynamic execution patterns
* infrastructure provisioning definitions
* large or outdated dependency graphs
* low testing confidence in critical services

Standard depth (depth=2) for normal analysis.
Light depth (depth=1) for low-risk or well-tested areas.

================================
AVAILABLE SCAN CATEGORIES
=========================

sast     — static analysis (semgrep, gosec, bandit, njsscan, brakeman)
sca      — software composition (trivy, grype, osvscanner)
secrets  — secret detection (gitleaks, trufflehog, detectsecrets)
iac      — infrastructure-as-code (checkov, tfsec, kics)
deps     — dependency audit (osvscanner, dependencycheck)

Only include categories that are relevant to the detected signals.
Assign priority: 1 = most critical, higher number = less urgent.

================================
PLAN OUTPUT CONTRACT
====================

Output ONLY valid JSON. No text outside the JSON.

{
  "system_summary": "2-3 sentences describing the system",
  "detected_signals": [
    {"type": "SECURITY_SIGNALS", "weight": 0.9, "reason": "..."}
  ],
  "risk_overview": "1-2 sentences on the main risk",
  "execution_plan": {
    "stages": [
      {
        "id": "stage_id",
        "parallel": true,
        "objective": "...",
        "tasks": [
          {
            "task_type": "tool",
            "target": "sast",
            "depth": 2,
            "priority": 1,
            "rationale": "..."
          }
        ]
      }
    ]
  },
  "expected_insights": "...",
  "confidence": 0.85
}`

// Orchestrator uses the LLM to produce a structured execution plan
// from project signals before any scanner runs.
// If LLM is unavailable, it falls back to a static default plan.
type Orchestrator struct {
	llm llm_domain.LLM
}

// NewOrchestrator creates a new Orchestrator.
func NewOrchestrator(llm llm_domain.LLM) *Orchestrator {
	return &Orchestrator{llm: llm}
}

// Plan produces an OrchestratorPlan from project signals.
// Falls back to a static safe plan if LLM is unavailable.
func (o *Orchestrator) Plan(ctx context.Context, signals subagents.ProjectSignals, req ScanRequest) (*OrchestratorPlan, error) {
	if o.llm == nil {
		return o.staticFallback(req), nil
	}

	prompt := o.buildPrompt(signals, req)

	var plan OrchestratorPlan
	err := o.llm.GenerateJSON(ctx, []llm_domain.Message{
		{Role: llm_domain.RoleSystem, Content: orchestratorSystemPrompt},
		{Role: llm_domain.RoleUser, Content: prompt},
	}, nil, &plan)

	if err != nil || len(plan.ExecutionPlan.Stages) == 0 {
		// LLM failed or returned empty plan — use static fallback
		return o.staticFallback(req), nil
	}

	return &plan, nil
}

// buildPrompt constructs the user message from project signals.
func (o *Orchestrator) buildPrompt(signals subagents.ProjectSignals, req ScanRequest) string {
	var sb strings.Builder

	sb.WriteString("## Project Context\n\n")
	sb.WriteString(fmt.Sprintf("Target: %s\n", req.TargetPath))
	sb.WriteString(fmt.Sprintf("Languages detected: %s\n", strings.Join(signals.Languages, ", ")))
	sb.WriteString(fmt.Sprintf("Frameworks: %s\n", join(signals.Frameworks, "none")))
	sb.WriteString(fmt.Sprintf("Has IaC: %v\n", signals.HasIaC))
	sb.WriteString(fmt.Sprintf("Has Docker: %v\n", signals.HasDocker))
	sb.WriteString(fmt.Sprintf("Has CI/CD: %v\n", signals.HasCI))
	sb.WriteString(fmt.Sprintf("Has Tests: %v\n", signals.HasTests))
	sb.WriteString(fmt.Sprintf("Total files: %d\n", signals.FileCount))

	if len(signals.RootFiles) > 0 {
		limit := 25
		if len(signals.RootFiles) < limit {
			limit = len(signals.RootFiles)
		}
		sb.WriteString(fmt.Sprintf("Root files: %s\n", strings.Join(signals.RootFiles[:limit], ", ")))
	}

	if req.MinSeverity != "" {
		sb.WriteString(fmt.Sprintf("\nUser requested minimum severity: %s\n", req.MinSeverity))
	}

	if len(req.Categories) > 0 {
		sb.WriteString(fmt.Sprintf("User requested categories: %s\n", strings.Join(req.Categories, ", ")))
	}

	sb.WriteString("\n## Execution Mode\nCLI — balanced actionable insights\n")
	sb.WriteString("\n## Task\nProduce the execution plan JSON. Only include scan categories relevant to the detected signals.")

	return sb.String()
}

// ExtractSubagentContexts converts a plan into SubagentContext list
// — the 4-tuple (Instruction, Context, Tools, Depth) per AOrchestra.
func (o *Orchestrator) ExtractSubagentContexts(plan *OrchestratorPlan) []SubagentContext {
	// Build shared context string from plan (passed to every subagent)
	sharedContext := buildSharedContext(plan)

	var contexts []SubagentContext
	seen := make(map[ScanCategory]bool)

	for _, stage := range plan.ExecutionPlan.Stages {
		for _, task := range stage.Tasks {
			if task.TaskType != "tool" {
				continue
			}
			cat := ScanCategory(task.Target)
			if !isValidCategory(cat) || seen[cat] {
				continue
			}
			seen[cat] = true

			contexts = append(contexts, SubagentContext{
				Category:    cat,
				Instruction: buildInstruction(cat, task, plan),
				Context:     sharedContext,
				Depth:       task.Depth,
				Priority:    task.Priority,
			})
		}
	}

	return contexts
}

// buildSharedContext creates the context string shared with all subagents.
func buildSharedContext(plan *OrchestratorPlan) string {
	var sb strings.Builder
	sb.WriteString("## System Context (from Orchestrator Analysis)\n\n")
	sb.WriteString(plan.SystemSummary)
	sb.WriteString("\n\n## Risk Overview\n")
	sb.WriteString(plan.RiskOverview)

	if len(plan.DetectedSignals) > 0 {
		sb.WriteString("\n\n## Key Risk Signals\n")
		for _, s := range plan.DetectedSignals {
			if s.Weight >= 0.6 {
				sb.WriteString(fmt.Sprintf("- [%s %.0f%%] %s\n", s.Type, s.Weight*100, s.Reason))
			}
		}
	}

	return sb.String()
}

// buildInstruction creates a targeted instruction for a specific subagent.
func buildInstruction(cat ScanCategory, task PlannedTask, plan *OrchestratorPlan) string {
	depthLabel := map[int]string{1: "light", 2: "standard", 3: "deep"}[task.Depth]
	if depthLabel == "" {
		depthLabel = "standard"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Run a %s %s scan (priority %d).\n", depthLabel, strings.ToUpper(string(cat)), task.Priority))
	sb.WriteString(fmt.Sprintf("Rationale: %s\n", task.Rationale))
	sb.WriteString(fmt.Sprintf("Objective: %s\n", task.Objective(plan)))

	if task.Depth >= 3 {
		sb.WriteString("\nEscalation: HIGH RISK ZONE — run all available scanners for this category, do not skip any.")
	} else if task.Depth == 1 {
		sb.WriteString("\nScope: Light scan — run only the primary scanner for this category.")
	}

	return sb.String()
}

// staticFallback returns a safe default plan when LLM is unavailable.
func (o *Orchestrator) staticFallback(req ScanRequest) *OrchestratorPlan {
	tasks := defaultTasks(req)
	return &OrchestratorPlan{
		SystemSummary:    "Static fallback plan — LLM unavailable.",
		RiskOverview:     "Running full scan with default settings.",
		ExpectedInsights: "Standard security findings across all categories.",
		Confidence:       0.5,
		ExecutionPlan: StagePlan{
			Stages: []ExecutionStage{
				{
					ID:        "full_scan",
					Parallel:  true,
					Objective: "Run all enabled scan categories",
					Tasks:     tasks,
				},
			},
		},
	}
}

// defaultTasks builds the default task list from a ScanRequest.
func defaultTasks(req ScanRequest) []PlannedTask {
	cats := defaultCategories(req)
	tasks := make([]PlannedTask, 0, len(cats))
	for i, cat := range cats {
		tasks = append(tasks, PlannedTask{
			TaskType:  "tool",
			Target:    string(cat),
			Depth:     2,
			Priority:  i + 1,
			Rationale: "Default scan — no LLM analysis available.",
		})
	}
	return tasks
}

// defaultCategories returns categories from the request or all if none specified.
func defaultCategories(req ScanRequest) []ScanCategory {
	if len(req.Categories) > 0 {
		cats := make([]ScanCategory, 0, len(req.Categories))
		for _, c := range req.Categories {
			if isValidCategory(ScanCategory(c)) {
				cats = append(cats, ScanCategory(c))
			}
		}
		return cats
	}
	return []ScanCategory{CategorySAST, CategorySCA, CategorySecrets, CategoryIaC, CategoryDeps}
}

// Objective resolves the stage objective for a task (uses stage objective as context).
func (t *PlannedTask) Objective(plan *OrchestratorPlan) string {
	for _, stage := range plan.ExecutionPlan.Stages {
		for _, task := range stage.Tasks {
			if task.Target == t.Target && task.TaskType == t.TaskType {
				return stage.Objective
			}
		}
	}
	return ""
}

func isValidCategory(cat ScanCategory) bool {
	_, ok := subagentScanners[cat]
	return ok
}

func join(s []string, fallback string) string {
	if len(s) == 0 {
		return fallback
	}
	return strings.Join(s, ", ")
}

// PlanSummary returns a human-readable summary of the plan for logging.
func PlanSummary(plan *OrchestratorPlan) string {
	b, _ := json.MarshalIndent(map[string]interface{}{
		"summary":    plan.SystemSummary,
		"risk":       plan.RiskOverview,
		"confidence": plan.Confidence,
		"stages":     len(plan.ExecutionPlan.Stages),
	}, "", "  ")
	return string(b)
}
