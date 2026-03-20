# Application Layer (`agent/internal/application`)

## Purpose
The `application` directory serves as the orchestration layer for the DuckOps Agent following Hexagonal Architecture principles. It wires together the core domain logic with the outbound adapters and exposes the application's capabilities to inbound drivers (like the CLI or UI).

## Files
- `application.go`: Defines the primary `App` container or Application logic sequence. It often acts as a struct that holds initialized dependencies and provides high-level use-case methods.
- `taskengine/`: Contains the task execution engine which coordinates the execution of background jobs, long-running agent tasks, or scheduled scans.

## Architectural Rules
- **Use Case Orchestration:** This layer coordinates multiple domain entities and adapters to fulfill a specific user request or system event.
- **Dependency Inversion:** It should depend on Interfaces (Ports) defined in the domain layer, not on concrete implementations in the `adapters` layer.
- **No I/O Logic:** It must not contain SQL queries, direct HTTP calls, or file system interactions—these must be delegated to adapters.
