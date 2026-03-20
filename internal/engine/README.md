# TUI & Kernel Bridge Engine (`agent/internal/engine`)

## Purpose
The `engine` directory provides a crucial bridging layer between the frontend User Interfaces (specifically the TUI) and the underlying AI `kernel`. It translates user inputs from the UI into standardized `domain.Task` objects and streams back the `kernel`'s execution events (like Thought processes, Token usage, and Tool executions) in a format the UI can consume.

## Files
- `engine.go`: Defines the `Engine` structural bridge, exposing interactive methods like `StreamChat` and `Chat` that intercept internal event pipelines to stream tokens and rationale back to the UI.
- `explain.go`: Provides logic for parsing and explaining specific operations or plans to the user transparently.
- `router.go` & `routing_policy.go`: Defines logic for routing user requests or execution flows to specific handlers or LLM models based on policy or intent.

## Architectural Rules
- **Mediation Layer:** The `Engine` is NOT the core intelligence (which lives in `agent` or `kernel`); it is purely a mediator that adapts internal complex event streams into developer/user-friendly streams.
- **TUI Coupling:** This module is allowed to embody logic specific to how users interact locally via the terminal, such as converting shell commands (`ls`, `pwd`) directly into tasks.
