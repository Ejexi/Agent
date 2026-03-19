# DuckOps — Architecture

## Overview

DuckOps is a DevSecOps AI agent written in Go. It runs locally, talks to Docker to execute security scanners in isolated containers, uses an LLM to orchestrate and interpret results, and surfaces findings through a conversational TUI.

---

## Layer Diagram

```
┌─────────────────────────────────────────────────┐
│                  CLI / TUI                       │
│  DuckOpsAgent (REPL)  ·  Bubble Tea TUI          │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│                 Agent Layer                      │
│  MasterAgent → Orchestrator → Subagents          │
│  (LLM-driven planning via AOrchestra pattern)    │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│                Kernel Layer                      │
│  Kernel · ToolRegistry · TaskEngine              │
│  (execution authority — all tool calls go here)  │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│                Warden Layer                      │
│  DockerWarden · ScannerService · Parsers         │
│  (container execution — knows nothing above)     │
└─────────────────────────────────────────────────┘
```

### Layer Rule (strict)

Each layer only imports from the layer directly below it. The Kernel never imports agent code. The Warden never imports Kernel code.

---

## Packages

### `internal/agent/`

| File | Responsibility |
|------|----------------|
| `master_agent.go` | 3-phase AOrchestra orchestration: Understand → Plan → Execute |
| `orchestrator.go` | LLM produces structured `OrchestratorPlan` JSON |
| `orchestrator_plan.go` | Plan types: `ExecutionStage`, `PlannedTask`, `SubagentContext` (4-tuple) |
| `scan_task_tracker.go` | Thread-safe tracker for parallel scan tasks |
| `duck_ops_agent.go` | Conversational REPL: parses intent, calls MasterAgent |
| `intent.go` | `ParseIntent()` — natural language → `Intent` struct |
| `report_agent.go` | Formats results, optional cloud push |
| `glamour.go` | All output rendered via `charmbracelet/glamour` |
| `types.go` | `ScanRequest`, `AggregatedResult`, `TaskPlan`, etc. |

### `internal/agent/subagents/`

| File | Responsibility |
|------|----------------|
| `port.go` | `SubagentPort` interface + `NewAllSubagents()` factory |
| `subagents.go` | 5 intelligent subagents: SAST, SCA, Secrets, IaC, Deps |
| `intelligent_base.go` | LLM-driven scanner selection + finding interpretation |
| `analyzer.go` | Fast filesystem walk → `ProjectSignals` (no LLM) |

### `internal/adapters/warden/`

| File | Responsibility |
|------|----------------|
| `docker_warden.go` | Container lifecycle, stdout capture, `WarmupImages` |
| `container_opts.go` | Security hardening: CapDrop ALL, no-new-privs, resource limits |
| `image_registry.go` | Scanner name → Docker image mapping |

### `shared/`

| Package | Responsibility |
|---------|----------------|
| `shared/scanner/domain` | `Finding`, `ScanResult`, `ScanStats`, `Location`, `Severity` |
| `shared/scanner/ports` | `ScannerPort`, `ScannerServicePort`, `ResultParserPort` |
| `shared/scanner/parsers/` | 19 parsers: trivy, semgrep, gitleaks, checkov, gosec, grype… |
| `shared/scanner/aggregator` | `ScannerService` — coordinates parsers + warden |
| `shared/events` | Canonical event types: `SubagentEvent`, `StreamEvent`, scan events |
| `shared/types` | `AppError`, `ErrorCode`, `Wrap`, `Newf` |
| `shared/ports` | `Logger`, `Field` |
| `shared/llm/domain` | `LLM` interface, `Message`, `GenerateOptions` |

---

## AOrchestra Execution Pattern

```
HandleScanRequest(req)
│
├── Phase 1 — Understand (no LLM, <50ms)
│     AnalyzeProject(path) → ProjectSignals
│     { languages, frameworks, hasIaC, hasDocker, hasCI, hasTests }
│
├── Phase 2 — Plan (LLM call)
│     Orchestrator.Plan(signals, req) → OrchestratorPlan
│     {
│       system_summary, detected_signals (weighted),
│       risk_overview, execution_plan (DAG stages),
│       expected_insights, confidence
│     }
│     ExtractSubagentContexts() → []SubagentContext
│     each = { Category, Instruction, Context, Depth, Priority }
│
└── Phase 3 — Execute (parallel goroutines)
      for each SubagentContext:
        selectByDepth(scanners, depth)  → depth=1: 1 scanner, 3: all
        goroutine → scannerSvc.RunScan() × N
      WaitForAll(30min) → aggregateResults()
```

### Depth levels

| Depth | Scanners used | Trigger |
|-------|--------------|---------|
| 1 | Primary scanner only | Low-risk area |
| 2 | First 2 scanners | Standard (default) |
| 3 | All scanners | High-risk: auth, crypto, IaC, large deps |

---

## Security Hardening

Every scanner runs in a Docker container with:

```
CapDrop:     ALL
SecurityOpt: no-new-privileges:true
Memory:      512MB
CPUQuota:    50% (50000)
PidsLimit:   100
Binds:       hostPath:/scan/workspace:ro  (read-only mount)
AutoRemove:  false  (we defer remove to ensure cleanup on panic)
```

The `defer ContainerRemove(Force: true)` runs even on panic or timeout.

---

## Error Handling

All errors use `shared/types.AppError`:

```go
// Creating
types.New(types.ErrCodeInternal, "message")
types.Newf(types.ErrCodeExecutionFailed, "timed out after %s", d)
types.Wrap(err, types.ErrCodeInternal, "context")

// Checking
types.FromError(err).Code  // → ErrorCode
```

Error codes follow the pattern `ERR_DUCKOPS_XXXX`.

---

## Event System

All cross-boundary events are defined in `shared/events/events.go`:

- `SubagentEvent` / `SubagentEventType` — emitted by SessionActor during LLM loops
- `StreamEvent` / `StreamEventType` — TUI progressive rendering
- `ScanRequestEvent` / `ScanResultEvent` — scanner bus messages

---

## Adding a New Scanner

1. Add parser in `shared/scanner/parsers/<category>/<name>/`
2. Implement `ResultParserPort` (Parse, ScannerName, GetScanCommand, SupportedFormats)
3. Register in `bootstrap.go` → `registerTools()` → parsers slice
4. Add to `subagentScanners` map in `agent/types.go`
5. Add Docker image in `adapters/warden/image_registry.go`
