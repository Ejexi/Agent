# Documentation (`agent/docs`)

## Purpose
The `docs` directory contains detailed, project-level documentation, architectural designs, and guides for developers or users of the DuckOps Agent.

## Contents

| Document | Description |
| --- | --- |
| `ADDING_SCANNER.md` | Guide on how to add a new security scanner to the DuckOps ecosystem. |
| `ARCHITECTURE.md` | High-level system architecture overview. |
| `CONFIGURATION.md` | Details on configuring the agent via `config.toml` or environment variables. |
| `SECURITY.md` | DuckOps security model, container hardening, and secrets handling. |
| `TUI_GUIDE.md` | Guide for interacting with the Terminal User Interface, including shortcuts and commands. |
| `architecture_guide.md`| Deep dive into the Hexagonal Architecture applied throughout the Go application. |

## Architectural Rules
- All new high-level non-code architectural decisions or user guides specific to the Agent should be placed here in Markdown format.
- Code documentation (package-level) should reside within the Go source files or nested READMEs where appropriate.
