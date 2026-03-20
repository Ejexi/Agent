# DuckOps CLI (`agent/cmd/duckops`)

## Purpose
The `cmd/duckops` directory contains the main entry point and CLI command definitions for the DuckOps DevSecOps Agent. It utilizes the Cobra framework to provide a robust command-line interface with various operational modes including an interactive REPL, TUI, and a background HTTP server.

## Files
- `root.go`: Defines the root `duckops` command, global flags (e.g., `--config`, `--verbose`, `--scan`, `--cli`), and handles the primary initialization sequence including configuration loading, bootstrapping, and launching the appropriate mode (TUI, REPL, or Scan).
- `config_cmd.go`: Manages configuration-related CLI commands.
- `log.go`: CLI command for viewing or streaming logs.
- `login.go`: CLI command for authenticating the CLI with external services or platforms.
- `main.go`: The minimal entry point that simply executes the root Cobra command.
- `run.go` / `runtime.go`: Handles execution and orchestration logic specific to the CLI running modes.
- `serve.go`: Command to start the background agent HTTP server.
- `setup_cmd.go`: Command for performing initial setup tasks.

## Architectural Rules
- This directory is purely a primary driver (inbound port) in Hexagonal Architecture.
- It should not contain business logic. Its sole responsibility is parsing CLI input, initializing the application container (via `bootstrap`), and invoking the corresponding application layer use cases or starting the UI/Server.
