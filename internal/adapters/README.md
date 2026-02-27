# adapters/

Concrete implementations of ports (Hexagonal Architecture adapters).

Adapters connect the agent to the external world. They implement interfaces defined in `internal/ports/` and contain **no business logic**.

## Subdirectories

| Directory                        | Implements Port     | Description                                   |
| -------------------------------- | ------------------- | --------------------------------------------- |
| [api/](api/)                     | —                   | HTTP API request/response handling            |
| [audit/](audit/)                 | `AuditPort`         | Session audit logging with backup             |
| [auth/](auth/)                   | —                   | Authentication adapter                        |
| [bootstrap/](bootstrap/)         | —                   | Composition Root — wires all dependencies     |
| [cli/](cli/)                     | —                   | CLI output adapter                            |
| [configsync/](configsync/)       | `ConfigSyncPort`    | Remote config sync (API Gateway)              |
| [elasticsearch/](elasticsearch/) | `LogDB`             | Elasticsearch adapter for scan logs           |
| [memory/](memory/)               | `MemoryPort`        | Generic memory adapter                        |
| [metadata/](metadata/)           | `MetadataDB`        | Vulnerability metadata adapter                |
| [rabbitmq/](rabbitmq/)           | `BusPort`           | RabbitMQ message bus adapter                  |
| [sandbox/](sandbox/)             | `SandboxPort`       | Container sandbox adapter                     |
| [secrets/](secrets/)             | `SecretScannerPort` | Secret detection adapter                      |
| [server/](server/)               | —                   | HTTP server adapter                           |
| [subagent/](subagent/)           | `SessionManager`    | Subagent lifecycle (Tracker, Session, Bridge) |
| [vectordb/](vectordb/)           | `VectorDB`          | Vector database adapter                       |
| [warden/](warden/)               | `WardenPort`        | Network sandbox proxy with Cedar policies     |

## Rules

- Adapters **must** implement ports.
- Adapters **must not** contain business logic or decision-making.
- Adapters are the **only** place where infrastructure libraries are imported.
