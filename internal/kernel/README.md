# kernel/

Execution authority of the DuckOps Agent.

## Golden Rule

> **The Kernel is the ONLY component allowed to execute tools.**

```go
// ✅ Correct
kernel.Execute(task)

// ❌ Forbidden
tool.Run()
```

## Files

| File             | Description                                                     |
| ---------------- | --------------------------------------------------------------- |
| `kernel.go`      | `Kernel` struct — coordinates Registry, Runtime, and Dispatcher |
| `registry.go`    | `Registry` — thread-safe tool registration and lookup           |
| `runtime.go`     | `Runtime` — single and parallel tool execution                  |
| `dispatcher.go`  | `Dispatcher` — listens on message bus, routes tasks to Runtime  |
| `kernel_test.go` | Unit tests for the Kernel                                       |

## Dependencies

The Kernel depends on:

- `internal/domain` — `Tool`, `Task`, `Result` types
- `internal/ports` — `BusPort`, `MemoryPort` interfaces
- `shared/llm/domain` — `LLMRegistry`
- `shared/ports` — `Logger`

It does **not** depend on any adapters or infrastructure.

## Execution Flow

```
RegisterTool(tool) → Registry stores tool
Execute(task)      → Runtime.Execute → Registry.Get → tool.ExecuteRaw
StartDispatcher()  → Dispatcher.Start → bus.Subscribe → Runtime.Execute → bus.Publish
```
