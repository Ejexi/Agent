# DevSecOps Agent - TODO List

This document outlines the next implementation steps required to complete the DevSecOps Agent Engine. All tasks strictly adhere to the established Hexagonal Architecture, Event-Driven Architecture, and internal rules defined in the `.agents/rules` and `.agents/workflows` directories.

---

## 1. Adapters Implementation (Infrastructure Layer)

_Reference: `adapters.md` & `memory.md`_

Adapters connect the system to the external world. They must implement the interfaces defined in `internal/ports/` and contain **zero business logic**.

- [ ] **Implement RabbitMQ Adapter**
  - Path: `internal/adapters/rabbitmq/adapter.go`
  - Responsibility: Fulfill `ports.BusPort` (Publish/Subscribe methods).
  - Rule Enforcement: Convert AMQP messages to `domain.Task` and route them to the Kernel's Dispatcher.
- [x] **Implement Config Module (Viper)**
  - Path: `internal/config/`
  - Responsibility: Manage `config.yaml` and OS environment variables parsing using 12-factor rules.
- [x] **Implement LLM Adapters & Registry**
  - Path: `internal/adapters/llm/`
  - Components: `Registry`, `OpenAI`, `Gemini`, `OpenRouter`
  - Responsibility: Fulfill `ports.LLMRegistry` and dynamic provider routing.
  - Rule Enforcement: Ensure strict Hexagonal boundaries, caching network clients, and fallback optimizations.
- [ ] **Implement Database Adapters (Memory)**
  - Path: `internal/adapters/memory/postgres.go` (and optionally `pgvector`, `elasticsearch`)
  - Responsibility: Fulfill `ports.MemoryPort` (Save/Load methods).
  - Rule Enforcement: Manage persistence outside the Kernel. The Kernel itself must remain stateless.
- [ ] **Implement gRPC Adapter (Optional/Future)**
  - Path: `internal/adapters/grpc/adapter.go`
  - Responsibility: Handle synchronous execution requests if required by other internal services.

---

## 2. Core Tools Implementation (Domain Layer)

_Reference: `tools.md` & `workflows/create_tool.md`_

Tools are the execution units of the agent. They must be **stateless**, deterministic, and only interact via passed `Ports`.

- [ ] **Implement Real `ScanTool`** (Currently a dummy)
  - Path: `internal/tools/scan/scan_tool.go`
  - Responsibility: Perform actual SAST/DAST scanning logic utilizing the LLM and Memory ports.
- [ ] **Implement `RemediationTool`**
  - Path: `internal/tools/remediation/remediation_tool.go`
  - Responsibility: Use the LLMPort to suggest or automatically apply security patches based on scan results.
- [ ] **Implement `QueryTool`**
  - Path: `internal/tools/query/query_tool.go`
  - Responsibility: Interface with the MemoryPort to retrieve past scan context or organizational knowledge base.

---

## 3. Entrypoints & Wiring (cmd Layer)

_Reference: `architecture.md`_

The `cmd` layer is the only place authorized to instantiate dependencies (Adapters) and inject them into the Kernel.

- [ ] **Create Event-Driven Worker Entrypoint**
  - Path: `cmd/agent-worker/main.go`
  - Setup: Initialize `RabbitMQAdapter`, `PostgresAdapter`, `LLMAdapter`.
  - Register Tools: Register `ScanTool`, `RemediationTool`, `QueryTool`.
  - Execution Flow: Run `kernel.StartDispatcher(ctx, "tasks.queue")` to start listening for async jobs.
- [ ] **Enhance CLI Entrypoint**
  - Path: `cmd/agent-cli/main.go`
  - Setup: Parse terminal arguments into a `domain.Task`.
  - Execution Flow: Fire directly using `kernel.Execute()` for synchronous, on-demand operations.

---

## 4. Hardening & Error Handling

_Reference: `AGENT.md`_

- [ ] **Integrate Telemetry and Tracing:** Attach OpenTelemetry context to our `Types.AppError` and Dispatcher logs.
- [ ] **Write Unit Tests:**
  - Mock Ports to test the Kernel.
  - Mock Ports to test individual Tools.
  - Test Adapters against actual (or containerized) infrastructure.
