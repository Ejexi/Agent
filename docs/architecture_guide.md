# DuckOps Agent — Architecture Overview & Development Guide

---

## 1️⃣ Executive Overview

This document explains:

- The main parts of the system
- How they relate to each other
- Why the architecture is designed this way
- Extracted architectural patterns
- A clear development guide for future work

The goal is to provide a **mental model** of the system so any contributor can understand it quickly and extend it safely.

---

## 2️⃣ High-Level Architecture

The system follows a layered + modular architecture with clear boundaries.

### Core Layers

1. **Entry Layer**
   - CLI (`cmd/duckops/`)
   - HTTP API (`internal/adapters/api/`)
   - Gateway (REST / SSE / JSON-RPC via `internal/adapters/server/`)

2. **Application Layer**
   - Use cases (tool execution, subagent workflows)
   - Orchestration logic (`internal/adapters/bootstrap/`, `internal/adapters/subagent/`)
   - Business rules (Kernel execution authority)

3. **Domain Layer (Core)**
   - Entities (`internal/domain/`)
   - Interfaces / Ports (`internal/ports/`)
   - Core contracts (`Tool`, `Task`, `Result`, security types)

4. **Infrastructure Layer (Adapters)**
   - Database (`internal/adapters/memory/`, `internal/adapters/metadata/`)
   - External APIs (`internal/adapters/configsync/`)
   - Message Bus (`internal/adapters/rabbitmq/`)
   - LLM providers (via `shared/llm/`)
   - Security (`internal/adapters/warden/`, `internal/adapters/secrets/`)

---

## 3️⃣ Main Components & Responsibilities

### 3.1 Agent Core (Kernel)

**Location:** `internal/kernel/`

Responsible for:

- **Execution authority** — the Kernel is the **only** component allowed to execute tools
- Tool Registry management
- Runtime dispatching
- Message bus integration

Why it exists:

- To isolate AI reasoning from infrastructure details
- To enforce the Golden Rule: `kernel.Execute(task)`, never `tool.Run()`

It depends only on **ports (interfaces)**, not concrete implementations.

**Key files:**
| File | Purpose |
|------|---------|
| `kernel.go` | Execution authority — coordinates Registry, Runtime, Dispatcher |
| `registry.go` | Thread-safe tool registration and lookup |
| `runtime.go` | Single and batch tool execution |
| `dispatcher.go` | Message bus listener — receives tasks, publishes results |

---

### 3.2 Orchestrator (Subagent System)

**Location:** `internal/adapters/subagent/`

Responsible for:

- Coordinating multi-step AI workflows
- Session lifecycle management
- Routing events between agents
- Spawning sub-agents with capability profiles

Why separate from Kernel?

- Keeps reasoning separate from workflow coordination
- Prevents fat-agent anti-pattern
- Kernel executes tools; Tracker manages agent lifecycles

**Key files:**
| File | Purpose |
|------|---------|
| `tracker.go` | Session lifecycle, implements `ports.SessionManager` |
| `session.go` | Individual subagent session state |
| `bridge.go` | `KernelBridge` adapter — connects subagent system to Kernel |
| `capability_registry.go` | Manages injected subagent profiles |
| `eventlog.go` | Event logging for subagent sessions |

---

### 3.3 Server Layer

**Location:** `internal/adapters/server/`, `internal/adapters/api/`

Responsible for:

- Exposing APIs (REST, SSE, JSON-RPC)
- Handling authentication (`internal/adapters/auth/`)
- Managing sessions
- Acting as system boundary

Relationship with Agent:

- Server calls Application layer
- Application layer drives Agent (Kernel)
- Agent never depends on Server

This ensures clean dependency direction.

---

### 3.4 Extensions / Tools (Plugins)

**Location:** `internal/tools/`

Responsible for:

- Adding tools dynamically
- Integrating external systems
- Each tool implements the `domain.Tool` interface

**Available tools:**
| Tool | Package | Purpose |
|------|---------|---------|
| Chat | `implementations/chat/` | LLM conversation via function calling |
| Echo | `implementations/echo/` | Testing / debugging echo tool |
| Scan | `implementations/scan/` | Security scanning (SAST, DAST, Secrets, etc.) |
| Subagent | `implementations/subagent/` | Spawn and resume sub-agents |
| Delegate | `implementations/delegate/` | Delegate tasks to capability-matched sub-agents |
| Kubernetes | `implementations/kubernetes/` | Kubernetes operations (planned) |
| VectorDB | `implementations/vectordb/` | Vector database operations (planned) |

---

## 4️⃣ Dependency Direction (Critical Rule)

All dependencies must follow this rule:

```
Infrastructure → Application → Domain
```

```
domain   → no dependencies
kernel   → domain only
ports    → domain only
adapters → ports only
tools    → ports + domain
cmd      → kernel only
```

**NEVER:**

- Domain importing infrastructure
- Application importing concrete adapters
- Tools accessing infrastructure directly

This enforces the **Dependency Inversion Principle**.

---

## 5️⃣ Extracted Architectural Patterns

### ✅ 1. Hexagonal Architecture (Ports & Adapters)

Core logic depends only on interfaces (`internal/ports/`).
Adapters (`internal/adapters/`) implement those interfaces.

Benefits:

- Easy testing with mocks
- Replaceable infrastructure
- Clean boundaries

---

### ✅ 2. Event-Driven Flow

Used for:

- Task dispatching via message bus (`Dispatcher` → RabbitMQ)
- Tool execution results published back to topics
- Agent state transitions logged via `EventLog`

Benefits:

- Decoupling between components
- Async scalability

---

### ✅ 3. Orchestrator Pattern

`Tracker` is the central workflow manager.
Separates coordination (session lifecycle) from execution (Kernel).

---

### ✅ 4. Composition Root Pattern

Object wiring happens in one place: `internal/adapters/bootstrap/`.
`FromTOML()` creates Kernel, LLM Registry, Tracker, and registers all tools.

Prevents scattered dependency creation.

---

### ✅ 5. Strategy Pattern

Used for:

- **LLM providers** — OpenAI, OpenRouter, Gemini via `LLMRegistry`
- **Memory strategies** — PostgreSQL, pgvector, Elasticsearch via ports
- **Tool execution** — Each tool is a strategy implementing `domain.Tool`

---

## 6️⃣ Memory Model

### Short-Term Memory

- Context of current session (subagent session state)
- Conversation history (managed by `SessionActor`)
- Temporary state

### Long-Term Memory

- **MetadataDB** (`internal/ports/metadatadb.go`) — PostgreSQL for vulnerability metadata
- **LogDB** (`internal/ports/logdb.go`) — Elasticsearch for raw scan logs
- **VectorDB** (`internal/ports/vectordb.go`) — pgvector for embeddings
- **MemoryPort** (legacy) — generic key-value fallback

Separation ensures:

- Performance (right storage for each use case)
- Clean responsibility
- Scalable memory growth

---

## 7️⃣ Security Architecture

### Warden (Network Sandbox)

**Port:** `ports.WardenPort` → **Adapter:** `internal/adapters/warden/`

- Transparent HTTP/HTTPS proxy with Cedar policy evaluation
- Runs entirely on-premises with mTLS
- No data leaves the environment without policy approval

### Secret Scanner

**Port:** `ports.SecretScannerPort` → **Adapter:** `internal/adapters/secrets/`

- Detects secrets in text before reaching any LLM
- Replaces with placeholders; restores at execution time
- Stateless and thread-safe

### Audit Logger

**Port:** `ports.AuditPort` → **Adapter:** `internal/adapters/audit/`

- Session audit logging with backup support
- SSH remote backup capability

---

## 8️⃣ Development Guide (Strict Rules)

### Rule 1: Respect Boundaries

Never cross architectural layers.

If you need something from another layer → define an interface in `internal/ports/`.

---

### Rule 2: No Direct Instantiation in Core

Core should not create concrete implementations.

Use dependency injection. All wiring happens in `internal/adapters/bootstrap/`.

---

### Rule 3: One Responsibility Per Module

If a module:

- Talks to DB **AND**
- Contains business logic

→ It is wrong. Split it.

---

### Rule 4: All External Systems Are Adapters

Database, HTTP clients, LLMs, message queues → adapters in `internal/adapters/`.

Never mix them into domain logic.

---

### Rule 5: Orchestrator Owns Workflows

- **Kernel** executes tools.
- **Tracker** coordinates agent lifecycles.
- **Server** exposes APIs.

Do not mix these responsibilities.

---

### Rule 6: Kernel Is the Only Executor

```go
// ✅ Correct
kernel.Execute(task)

// ❌ Forbidden
tool.Run()
```

---

## 9️⃣ How to Add New Features Safely

### Adding a New Tool

1. Define tool schema using `domain.ToolSchema`
2. Implement `domain.Tool` interface in `internal/tools/implementations/<name>/`
3. Optionally use `base.TypedToolBase` for type-safe parameter parsing
4. Register in `bootstrap.registerTools()` in `internal/adapters/bootstrap/`
5. Tool is automatically available to the Kernel

---

### Adding a New Workflow

1. Define use case in Application layer
2. Implement orchestration logic in subagent system
3. Expose through Server/CLI

---

### Adding a New LLM Provider

1. Implement `shared/llm/domain.LLMProvider` interface
2. Add provider config in `~/.duckops/config.toml`
3. Register in `bootstrap.buildLLMRegistry()`

No core changes required.

---

### Adding a New Adapter

1. Define port interface in `internal/ports/`
2. Implement adapter in `internal/adapters/<name>/`
3. Inject via `internal/adapters/bootstrap/`

---

## 🔟 Anti-Patterns to Avoid

| ❌ Anti-Pattern       | Why It's Bad                                  |
| --------------------- | --------------------------------------------- |
| Fat Agent             | Agent should not contain infrastructure logic |
| God Orchestrator      | Tracker should coordinate, not decide         |
| Mixed config systems  | Use TOML config exclusively                   |
| Circular dependencies | Breaks dependency direction rule              |
| Hidden global state   | All state must be explicit and injected       |
| Direct tool execution | Only Kernel may execute tools                 |

---

## 1️⃣1️⃣ Testing Strategy

### Unit Tests

- Test Domain with mocks
- No infrastructure in core tests
- Example: `kernel_test.go`, `eventlog_test.go`, `warden_test.go`

### Integration Tests

- Test adapters separately
- Mock ports for adapter testing

### E2E Tests

- Test full workflow through CLI or API
- Validate: CLI → Kernel → Runtime → Tool → Result

---

## 1️⃣2️⃣ Execution Flow

```
CLI → Bootstrap → Kernel → Runtime → Tool → Result
                     ↕
              Message Bus (RabbitMQ)
                     ↕
               Dispatcher → Worker → Result
```

---

## 1️⃣3️⃣ Configuration

Configuration is loaded from `~/.duckops/config.toml` via `internal/config/`.

Supports:

- Named profiles (e.g., `default`, `super`)
- Multiple LLM providers per profile
- Warden sandbox settings
- Secret scanning settings
- Audit logging settings
- Agent modes: `Stand Duck ` | `super`

---

## 1️⃣4️⃣ Mental Model Summary

Think of the system as:

| Concept              | System Component              |
| -------------------- | ----------------------------- |
| Brain rules          | `internal/domain/`            |
| Decision coordinator | `internal/kernel/`            |
| Workflow manager     | `internal/adapters/subagent/` |
| Hands                | `internal/adapters/`          |
| Gateway              | `cmd/duckops/` + Server       |

**The brain never depends on the hands.**

---

## 1️⃣5️⃣ Final Philosophy

This project is designed for:

- **Scalability** — event-driven, message bus, distributed agents
- **Replaceability** — any adapter can be swapped via ports
- **Testability** — core logic testable without infrastructure
- **Long-term maintainability** — strict boundaries, single responsibility

Every change should preserve:

- Clean boundaries
- Dependency direction
- Single responsibility

**If a change breaks those — redesign it.**
