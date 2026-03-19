---
name: aorchestra-pattern
description: AOrchestra multi-agent orchestration pattern — Master→Subagent 4-tuple delegation, depth-adaptive execution, structured planning
---

# AOrchestra Pattern

AOrchestra is DuckOps' multi-agent orchestration pattern. The Master Agent acts as a planning engine, not a task runner. It produces a structured execution plan before spawning any subagents.

## Core Principle

> The master reasons first. Subagents execute with full context.

Contrast with naive orchestration:
```
❌ Naive: Master → "run SAST" → subagent blindly runs semgrep
✅ AOrchestra: Master → analyse project → plan → subagent gets (Instruction + Context + Tools + Depth)
```

## The 4-Tuple

Every subagent receives:

```go
type SubagentContext struct {
    Category    ScanCategory  // what category to scan
    Instruction string        // what to focus on (from plan)
    Context     string        // system understanding (shared)
    Depth       int           // 1=light, 2=standard, 3=deep
    Priority    int           // 1=most critical
}
```

**Example instruction for SAST on a Go auth service:**
```
Run a deep SAST scan (priority 1).
Rationale: Authentication logic detected in /internal/auth — high risk zone.
Objective: Identify injection vectors, auth bypasses, and privilege escalation paths.

Escalation: HIGH RISK ZONE — run all available scanners for this category.
```

## Depth Levels

Depth is set by the orchestrator based on risk signals:

| Depth | Scanners | Trigger |
|-------|---------|---------|
| 1 | Primary only | Low-risk, low-complexity module |
| 2 | First 2 | Standard — most components |
| 3 | All | Auth, crypto, IaC, large dep graphs |

**Escalation triggers (depth → 3):**
- Authentication or authorization logic
- Cryptographic usage patterns  
- External API exposure
- Infrastructure provisioning definitions
- Low test coverage in critical services

## Execution Flow

```
Phase 1 — Understand (no LLM, <50ms)
  AnalyzeProject(path) → ProjectSignals
  { languages, frameworks, hasIaC, hasDocker, hasCI, hasTests, fileCount }

Phase 2 — Plan (1 LLM call)
  Orchestrator.Plan(signals) → OrchestratorPlan {
    system_summary
    detected_signals: [{ type, weight, reason }]
    risk_overview
    execution_plan: { stages: [{ tasks: [{ target, depth, priority }] }] }
    confidence
  }

Phase 3 — Execute (parallel goroutines)
  for each SubagentContext (sorted by priority):
    selectByDepth(allowedScanners, depth)
    goroutines → scannerSvc.RunScan() × N
  WaitForAll → aggregateResults
```

## Fallback Strategy

If the LLM is unavailable or returns an invalid plan:
1. `staticFallback()` produces a safe default plan (depth=2, all categories)
2. No error is surfaced to the user — scan proceeds
3. Logged internally for observability

## Adding a New Scan Domain

1. Define the category constant in `agent/types.go:subagentScanners`
2. Add scanners to the allowed list (least-privilege)
3. Write a system prompt following the pattern in `subagents/subagents.go`
4. Implement `SubagentPort` (Name, DefaultScanners, Run)
5. Add to `NewAllSubagents()` factory

## Orchestrator System Prompt Design

The system prompt must instruct the LLM to:
- Output **only** valid JSON (no prose outside the schema)
- Follow the `OrchestratorPlan` schema exactly
- Assign realistic depth (1/2/3) based on risk signals
- Only include categories relevant to the detected stack
- Use `confidence` 0.0–1.0 honestly
