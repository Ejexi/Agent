# DevSecOps AI Agent: Project Context

## 1. Project Identity

- **Name**: DevSecOps AI Agent
- **Description**: A modular, extensible AI agent designed for DevSecOps tasks. It utilizes a **Hexagonal Architecture** (Ports & Adapters) to decouple core logic from external tools and interfaces.
- **Primary Language**: Go (1.25+)

## 2. Tech Stack & Dependencies

- **Configuration**: [`viper`](github.com/spf13/viper) - Handles YAML config and environment variables.
- **Logging**: [`zap`](go.uber.org/zap) - Structured, high-performance logging.
- **CLI Framework**: [`cobra`](github.com/spf13/cobra) - Command-line interface management.
- **AI Client**: [`openai-go`](github.com/openai/openai-go/v2) - Interaction with LLMs.
- **Test Framework**: Native Go testing + `testify` (implied by typical Go stacks, though simple `testing` is core).

<!--
## RAG Configuration (Planned)

rag:
  enabled: true
  chunk_size: 1000
  chunk_overlap: 200
  embedding_model: "text-embedding-3-small"
  vector_db:
    type: "postgres"
    connection_string: "${POSTGRES_CONN_STRING}"
-->

## 3. Architecture Overview

The system follows **Hexagonal Architecture** to ensure maintainability and testability.

### Layers

1.  **Domain (Core)**:
    - Located in `internal/core` and `internal/agent`.
    - Contains business rules, state management (`Memory`), and foundational services (`Config`, `Logger`).
    - _Dependency Rule_: Depends on nothing; purely internal.
2.  **Ports (Interfaces)**:
    - Located in `internal/tools/base`.
    - Defines the `Tool` interface and `BaseTool` struct.
    - _Dependency Rule_: Defined by the Domain/Application.
3.  **Adapters (Infrastructure)**:
    - **Driving**: `cmd/agent` (CLI entry point that drives the application).
    - **Driven**: `internal/tools/implementations` (Concrete tools like FileSystem, Echo, etc.).
    - _Dependency Rule_: Depends on Domain and Ports.

## 4. Key Directories Map

| Path                  | Purpose                  | Key Artifacts                                                   |
| :-------------------- | :----------------------- | :-------------------------------------------------------------- |
| **`cmd/`**            | Application Entry Points | `agent/main.go` (Main executable)                               |
| **`internal/core/`**  | Core Infrastructure      | `config/*.go` (Viper), `logger/*.go` (Zap)                      |
| **`internal/agent/`** | Agent Logic              | `memory/memory.go` (Context window/history)                     |
| **`internal/tools/`** | Tooling System           | `base/` (Interfaces), `registry/` (Manager), `implementations/` |
| **`configs/`**        | Configuration Assets     | `config.yaml` (Runtime settings)                                |

## 5. Core Abstractions & Usage

### Tool System

The agent's capabilities are extensible via the Tool Registry.

- **Base**: All tools must implement the `base.Tool` interface and typically embed `base.BaseTool`.
- **Registry**: `internal/tools/registry` is a singleton-like store for all available tools.
- **Adding a Tool**:
  1.  Create `internal/tools/implementations/<name>/<name>.go`.
  2.  Implement `Execute` and `Validate`.
  3.  Register via `registry.Register()` in the main startup sequence.

### Memory / Context

- **Implementation**: `internal/agent/memory`.
- **Function**: Stores conversation history (User/System/Assistant messages).
- **Concurrency**: Thread-safe access via Mutex.

### Configuration

- **Loader**: `internal/core/config.Load()`.
- **Source**: Reads from configuration files (YAML) and Environment Variables.

## 6. Project Conventions

- **Logging**: ALWAYS use the structured Logger (`logger.Info`, `logger.Error`) instead of `fmt.Println`.
- **Error Handling**: Return errors to the caller. Panic only on startup (invariant violation).
- **Modularity**: Keep tool logic isolated in `implementations`. Core logic stays in `internal/core`.

## 7. Operational Commands

- **Run Agent**: `go run cmd/agent/main.go`
- **Install Dependencies**: `go mod download`
- **Test**: `go test ./...`
