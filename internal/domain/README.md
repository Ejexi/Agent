# Domain Entities (`agent/internal/domain`)

## Purpose
The `domain` directory contains the core enterprise business rules and entities for the DuckOps system. In Hexagonal Architecture, this is the innermost layer. It defines fundamental types and behaviors independent of external systems or frameworks.

## Subdirectories & Files
- Core domain objects: `os_task.go`, `tool.go`, `hook.go`, `events.go`.
- Sub-domains for specialized interactions: `mcp/` (Model Context Protocol models), `rag/` (Retrieval-Augmented Generation models), `security/`, `orchestration/`, `subagent/`.

## Architectural Rules
- **No External Dependencies:** Domain entities must not depend on any outer layers (no database driver imports, no UI code, no specific web frameworks).
- **Core Entities:** Models defined here represent universal concepts within DuckOps, utilized by the application layer.
