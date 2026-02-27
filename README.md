# 🦆 DuckOps Agent

The autonomous DevSecOps execution engine.

---

## Overview

The **DuckOps Agent** is a production-grade, modular worker engine built in Go. It executes security and DevSecOps tasks following a strict **Kernel–Policy Architecture** on top of Hexagonal (Ports & Adapters) principles.

> **Golden Rule:** The Kernel is the **only** component allowed to execute tools.

---

## Architecture

The agent follows a layered architecture with strict dependency direction:

```
Infrastructure → Application → Domain
```

| Layer        | Package              | Description                                                                           |
| ------------ | -------------------- | ------------------------------------------------------------------------------------- |
| **Domain**   | `internal/domain/`   | Core entities (`Tool`, `Task`, `Result`), security types. Zero external dependencies. |
| **Kernel**   | `internal/kernel/`   | Execution authority — Registry, Runtime, Dispatcher. Depends only on domain.          |
| **Ports**    | `internal/ports/`    | Interfaces for all external systems (LLM, Memory, MessageBus, Warden, etc.)           |
| **Adapters** | `internal/adapters/` | Concrete implementations of ports. No business logic.                                 |
| **Tools**    | `internal/tools/`    | Stateless execution units implementing `domain.Tool`.                                 |
| **Config**   | `internal/config/`   | TOML configuration (`~/.duckops/config.toml`).                                        |
| **Entry**    | `cmd/duckops/`       | CLI commands (`run`, `serve`, `login`, `config`).                                     |

> 📖 Full architecture documentation: [docs/architecture_guide.md](docs/architecture_guide.md)

---

## Directory Structure

```
agent/
├── cmd/duckops/                    # CLI entry point (main, run, serve, login)
├── docs/                           # Architecture guide & documentation
├── internal/
│   ├── domain/                     # Core types & interfaces
│   │   ├── rag/                    #   RAG domain types
│   │   ├── security/              #   Warden, secrets, audit types
│   │   └── subagent/              #   Subagent lifecycle types
│   ├── kernel/                     # Execution authority
│   │   ├── kernel.go              #   Kernel (Registry + Runtime + Dispatcher)
│   │   ├── registry.go            #   Thread-safe tool registration
│   │   ├── runtime.go             #   Single & batch tool execution
│   │   └── dispatcher.go          #   Message bus task listener
│   ├── ports/                      # Interface definitions (13 ports)
│   ├── config/                     # TOML config loading
│   ├── adapters/                   # Infrastructure implementations
│   │   ├── bootstrap/             #   Composition Root (dependency wiring)
│   │   ├── subagent/              #   Session lifecycle (Tracker, Bridge)
│   │   ├── rabbitmq/              #   Message bus adapter
│   │   ├── elasticsearch/         #   Log storage adapter
│   │   ├── warden/                #   Network sandbox proxy (Cedar)
│   │   ├── secrets/               #   Secret scanner adapter
│   │   ├── audit/                 #   Audit logging adapter
│   │   ├── server/                #   HTTP server adapter
│   │   └── ...                    #   More adapters
│   └── tools/                      # Tool implementations
│       ├── base/                  #   TypedToolBase[P] generic helper
│       └── implementations/       #   Concrete tools
│           ├── chat/              #     LLM conversation
│           ├── scan/              #     Security scanning
│           ├── subagent/          #     Spawn/resume sub-agents
│           ├── delegate/          #     Capability-matched delegation
│           ├── echo/              #     Testing/debugging
│           ├── kubernetes/        #     K8s operations (planned)
│           └── vectordb/          #     Vector DB operations (planned)
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
└── go.sum
```

---

## Execution Flow

```
CLI → Bootstrap → Kernel → Runtime → Tool → Result
                    ↕
             Message Bus (RabbitMQ)
                    ↕
              Dispatcher → Worker → Result
```

---

## Tool Interface

Every tool implements `domain.Tool`:

```go
type Tool interface {
    Name() string
    Schema() ToolSchema
    ExecuteRaw(ctx context.Context, input map[string]interface{}) (Result, error)
}
```

### Rules for Tools

1. **Stateless** — No internal mutable state between runs.
2. **No direct I/O** — Use injected ports for all external access.
3. **Error handling** — Return `types.New` or `types.Wrap` wrapped errors.
4. **Deterministic** — Same input → same output (unless explicitly nondeterministic).

---

## Adding a New Tool

1. Create a package under `internal/tools/implementations/<name>/`.
2. Implement `domain.Tool` (or embed `base.TypedToolBase[P]` for type safety).
3. Register in `internal/adapters/bootstrap/bootstrap.go` → `registerTools()`.
4. Add a `README.md` to the new directory.
5. Write unit tests (`*_test.go`).

---

## Configuration

Config lives at `~/.duckops/config.toml`. Auto-created on first run.

```toml
[profiles.default]
provider = "openrouter"

[profiles.default.providers.openrouter]
type = "openrouter"
base_url = "https://openrouter.ai/api/v1"
[profiles.default.providers.openrouter.auth]
type = "env"
key = "OPENROUTER_API_KEY"

[settings]
agent_mode = "standalone"    # "standalone" or "super"
server_addr = ":8090"
```

---

## Testing

```bash
go test ./...
```

- **Unit tests** — Domain and kernel tested with mocks (no infrastructure).
- **Integration tests** — Adapters tested separately.
- **Linting** — `golangci-lint` and `go vet` enforced in CI.

---

## Dependencies

This module depends on **[DuckOps Shared](https://github.com/SecDuckOps/shared)** which provides the shared `AppError` system, LLM ports, and event types.

```bash
# Local development
replace github.com/SecDuckOps/shared => ../shared
```

The Agent interacts with the **[Server](https://github.com/SecDuckOps/server)** asynchronously via RabbitMQ. It is a "Remote Procedure Call" target for the Server's Orchestrator and remains completely unaware of the Server's persistence or state machine.

---

## Contributing

1. Fork the repository.
2. Create a feature branch (`git checkout -b feat/your-feature`).
3. Follow **Hexagonal Architecture** guidelines — keep domain pure.
4. Write tests for every new function.
5. Run `go fmt` and `golangci-lint` locally.
6. Submit a Pull Request with a clear description.

---

## License

MIT License — see `LICENSE` file.

---

_DuckOps Agent: Secure, Isolated, Autonomous._ 🦆
