# domain/

Core domain layer — entities, value objects, and contracts.

## Files

| File          | Description                                                                                   |
| ------------- | --------------------------------------------------------------------------------------------- |
| `tool.go`     | `Tool` interface, `TypedTool` generic, `ToolSchema` metadata                                  |
| `task.go`     | `Task` struct — represents a unit of work for the Kernel                                      |
| `result.go`   | `Result` struct — outcome of tool execution                                                   |
| `events.go`   | Security scanning domain: `ScanRequest`, `ScanResult`, `Vulnerability`, severity/status enums |
| `provider.go` | Provider-related domain types                                                                 |

## Subdirectories

| Directory              | Description                                             |
| ---------------------- | ------------------------------------------------------- |
| [rag/](rag/)           | RAG (Retrieval-Augmented Generation) domain types       |
| [security/](security/) | Security domain: network policies, secrets, mTLS config |
| [subagent/](subagent/) | Subagent domain types and lifecycle definitions         |

## Rules

- **Zero external dependencies** — domain imports nothing from adapters or infrastructure.
- All interfaces consumed by the Kernel and tools are defined here.
- This is the innermost layer in the Hexagonal Architecture.
