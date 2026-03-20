# DuckOps Agent Configuration (`agent/internal/config`)

## Purpose
The `config` directory is responsible for defining, parsing, and managing the configuration of the DuckOps Agent. It handles reading settings from the environment, configuration files (e.g., typically `config.toml` or similar), and CLI arguments.

## Architectural Rules
- **Centralized Configuration:** This package should be the single source of truth for configuration structures. Other modules should depend on these structures rather than parsing environment variables or files directly.
- **Immutability:** Once loaded, configuration should ideally be treated as read-only throughout the application lifecycle.
- **Dependency Injection:** The loaded configuration object (or relevant parts of it) should be injected into the services and adapters that require it, avoiding global state where possible.

## Files
- `duckops_config.go`: Defines the core configuration structures (e.g., `DuckOpsConfig`, potentially integrating global options, server settings, and MCP connection settings) and the logic required to populate them from external sources.
