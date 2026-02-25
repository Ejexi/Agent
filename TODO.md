✅ Phase 1 — Agent Core (مكتمل)
EventLog (ring buffer + fan-out SSE)
Session State Machine (RunState + RunID)
SessionActor → EventLog emission
SSE streaming (subscribe/unsubscribe/replay)
ToolCall events from model
✅ Phase 2 — Standalone Binary (مكتمل)
Cobra CLI (duckops, duckops run, serve, login, config, version)
TOML config (~/.duckops/config.toml) — profiles, providers, warden, settings
Auth store (~/.duckops/auth.toml) — API key, 0600 permissions
API client (

internal/api/client.go
)
Sandbox interface + StubSandbox
Server + REPL تشتغل مع بعض في process واحد
🔲 Phase 3 — Tool Execution & Scanner Workers
SAST Worker tool
DAST Worker tool
Secrets Scanner tool
Container Scanner tool
Dependency Scanner tool
IaC Scanner tool
Tool results → EventLog pipeline
🔲 Phase 4 — RAG Remediation Pipeline
pgvector integration (semantic search)
Embedding generation for vulnerabilities
RAG prompt builder (vuln + context chunks → LLM)
Remediation suggestion output
Confidence scoring
🔲 Phase 5 — Server-Side (Control Plane)
Auth Service (JWT/OAuth2)
Orchestrator (task routing)
Policy Engine
Result Processor → PostgreSQL + Elasticsearch + pgvector
RabbitMQ message bus integration
API Gateway (REST + SSE + JSON-RPC)
Query Service
Notification Service (Slack/Discord/Email)
🔲 Phase 6 — Human-in-the-Loop (Approval Gate)
Fix proposal submission
Security Engineer review dashboard
Approve/Reject flow
Apply fix → Orchestrator
🔲 Phase 7 — Feedback Loop
Feedback Processor
Approved fix → positive embedding in pgvector
Rejected fix + reason → negative embedding
Next RAG cycle uses both good + bad patterns
Audit logging (PostgreSQL)
🔲 Phase 8 — Dashboard (Frontend)
Web dashboard
Agent monitoring
Scan results viewer
Approval workflow UI
Real-time SSE event stream
Analytics & reporting
🔲 Phase 9 — Distributed Agents
Agent registration & heartbeat
Multi-agent orchestration
Docker sandbox (DockerSandbox adapter)
Container-per-subagent isolation
mTLS agent-to-server auth
Warden guardrails
🔲 Phase 10 — TUI
Terminal UI (bubbletea أو مشابه)
Interactive agent management
Real-time scan progress
Session viewer