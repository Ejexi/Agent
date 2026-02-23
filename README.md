# DuckOps Agent: The Autonomous Execution Engine

## Overview

The **DuckOps Agent** is a high‑performance, stateless worker engine designed to execute security and DevSecOps tasks in isolated environments. It follows a strict **Kernel‑Policy Architecture** built on Hexagonal principles, ensuring clear separation of concerns, testability, and easy extensibility.

---

## Architecture

The project is organized into the following layers (see `architecture.md` for details):

- **Domain** – Core business types (`Tool`, `Task`, `Result`). No external dependencies.
- **Kernel** – Orchestrates execution. Depends only on the domain.
  - `registry.go` – Registers all available tools.
  - `runtime.go` – Wraps tool execution with panic recovery, metrics, and logging.
  - `dispatcher.go` – Routes incoming tasks from the MessageBus (RabbitMQ) to the kernel.
- **Ports** – Interfaces for external systems (LLM, Memory, Filesystem, etc.).
- **Adapters** – Implement ports (RabbitMQ, gRPC, DB, LLM). Contain **no business logic**.
- **Tools** – Stateless execution units that implement the `domain.Tool` interface and use only ports.
- **Cmd** – CLI entry point that invokes `kernel.Execute(task)`.

---

## Directory Structure

```
Agent/
├─ cmd/                # CLI entry points
├─ domain/             # Core types and interfaces
├─ kernel/             # Registry, Runtime, Dispatcher
├─ ports/              # Port interfaces (LLM, Memory, FS, etc.)
├─ adapters/           # Implementations of ports
│   ├─ rabbitmq/       # RabbitMQ adapter
│   ├─ grpc/           # gRPC adapter
│   └─ llm/            # LLM adapter
├─ tools/              # Stateless tool implementations
│   ├─ scan/           # Example: SecurityScanner
│   └─ remediation/   # Example: RemediationTool
└─ internal/           # Shared utilities (logging, errors)
```

---

## Core Components

### Registry (`kernel/registry.go`)

- Maintains a map of tool name → `domain.Tool` instance.
- Populated during bootstrap by each tool calling `registry.Register(name, tool)`.

### Runtime (`kernel/runtime.go`)

- Executes a tool inside a safe wrapper.
- Handles panic recovery, logs execution time, and returns a structured `domain.Result`.

### Dispatcher (`kernel/dispatcher.go`)

- Listens on the **MessageBus** (RabbitMQ) for incoming tasks.
- Deserialises the payload and forwards it to the kernel.

---

## Tool Development Guide

Every tool must implement the following interface (see `domain/tool.go`):

```go
type Tool interface {
    Name() string
    Run(ctx context.Context, task Task) (Result, error)
}
```

### Golden Rules for Tools

1. **Stateless** – No internal mutable state between runs.
2. **No direct I/O** – Use injected ports for filesystem, LLM, memory, etc.
3. **Error Handling** – Return `types.New` or `types.Wrap` wrapped `AppError` objects.
4. **Deterministic** – Given the same input, the tool should produce the same output (unless explicitly nondeterministic, e.g., LLM).

### Example Implementation

```go
type SecurityScanner struct {
    llm ports.LLM // injected port
}

func (s *SecurityScanner) Name() string { return "security-scan" }

func (s *SecurityScanner) Run(ctx context.Context, task domain.Task) (domain.Result, error) {
    target, ok := task.Args["target"].(string)
    if !ok {
        return domain.Result{}, types.New(types.ErrCodeInvalidInput, "missing target path")
    }
    finding, err := s.llm.Generate(ctx, "Scan this: "+target)
    if err != nil {
        return domain.Result{}, types.Wrap(err, types.ErrCodeToolFailed, "llm analysis failed")
    }
    return domain.Result{Success: true, Data: finding}, nil
}
```

---

## Adding a New Tool

1. Create a new package under `tools/`.
2. Implement the `Tool` interface.
3. Register the tool in `kernel/registry.go` during bootstrap.
4. Write unit tests in the same package (`*_test.go`).
5. Add any required port interfaces in `ports/` and adapters if needed.
6. Update `README.md` (this file) with a short description under **Available Tools**.

---

## Testing

- Run all unit tests: `go test ./...`
- Use the `mock` adapters in `adapters/mock/` to stub external services.
- CI pipeline runs `golangci-lint` and `go vet` to enforce code quality.

---

## Contributing

1. Fork the repository.
2. Create a feature branch (`git checkout -b feat/your-feature`).
3. Follow the **Hexagonal Architecture** guidelines – keep domain pure.
4. Write tests for every new function.
5. Run `go fmt` and `golangci-lint` locally.
6. Submit a Pull Request with a clear description and link to the relevant issue.

---

## License

MIT License – see `LICENSE` file.

---

_DuckOps Agent: Secure, Isolated, Autonomous._
