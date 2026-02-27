# adapters/cli/

CLI output adapter for the DuckOps Agent.

## Purpose

Handles terminal output formatting, role-tagged message display, and interactive CLI concerns.

## Rules

- No business logic — only terminal I/O formatting.
- Used by `cmd/duckops/` commands.
