# Adapters Layer (`agent/internal/adapters`)

## Purpose
The `adapters` directory contains all the secondary (outbound) and some primary (inbound) driver implementations for the DuckOps Agent, adhering to Hexagonal (Ports and Adapters) architecture. This is where the application interfaces with the outside world: databases, third-party APIs, file systems, MCP servers, and other infrastructure concerns.

## Subdirectories (Key Implementations)
- `api/`, `server/`, `events/`: Primary adapters handling inbound HTTP/event requests.
- `cli/`: Components related to supporting specific CLI-based output or legacy terminal UI.
- `audit/`, `security/`, `sarif/`, `warden/`: Security and compliance-focused secondary adapters to perform scanning, parsing SARIF, or communicating with external security engines.
- `mcp/`: Adapters for communicating with Model Context Protocol (MCP) servers (like `duckops-mcp-server` or `server-filesystem`).
- `executor/`: Adapters for running terminal commands or external processes safely.
- `auth/`, `configsync/`, `hooks/`, `bootstrap/`: Infrastructure-level adapters handling authentication, configuration synchronization, system hooks, and dependency injection wiring.
- `translator/`, `subagent/`: Adapters dealing with complex transformations or external agent delegation.

## Architectural Rules
- **Infrastructure Code Only:** Business rules or agent planning logic MUST NOT reside here.
- **Implement Ports:** Every adapter in this directory should implement an interface defined in the `domain` or `ports` directory.
- **Isolate Dependencies:** Third-party library integrations (like Docker client, GitHub API clients, specific DB drivers) are restricted to this layer.
