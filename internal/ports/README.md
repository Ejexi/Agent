# ports/

Interface definitions (Hexagonal Architecture ports).

All adapters must implement these interfaces. Core logic depends **only** on these ports, never on concrete implementations.

## Files

| File                 | Interface           | Description                                                   |
| -------------------- | ------------------- | ------------------------------------------------------------- |
| `memory.go`          | `MemoryPort`        | Generic key-value memory (deprecated — prefer specific ports) |
| `metadatadb.go`      | `MetadataDB`        | PostgreSQL for vulnerability metadata                         |
| `logdb.go`           | `LogDB`             | Elasticsearch for raw scan logs                               |
| `vectordb.go`        | `VectorDB`          | pgvector for embeddings and similarity search                 |
| `message_bus.go`     | `BusPort`           | Message bus (RabbitMQ) — publish/subscribe                    |
| `execution.go`       | `ExecutionPort`     | Tool execution abstraction                                    |
| `sandbox.go`         | `SandboxPort`       | Container sandbox for isolated execution                      |
| `session_manager.go` | `SessionManager`    | Subagent session lifecycle                                    |
| `subagent.go`        | `SubagentPort`      | Subagent spawn/resume contracts                               |
| `warden.go`          | `WardenPort`        | Network sandbox proxy with Cedar policies                     |
| `secrets.go`         | `SecretScannerPort` | Secret detection and substitution                             |
| `audit.go`           | `AuditPort`         | Session audit logging                                         |
| `config_sync.go`     | `ConfigSyncPort`    | Remote configuration synchronization                          |

## Rules

- Ports depend only on `internal/domain`.
- All external system interactions are defined here as interfaces.
- Adapters in `internal/adapters/` implement these ports.
