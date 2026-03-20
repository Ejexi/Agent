# Context Management (`agent/internal/context`)

## Purpose
The `context` directory manages the localized state, variables, and execution context for the running agents and workflows. This involves injecting relevant project data, configuration context, and tracking session-specific information during interaction with the LLM.

## Files
- `local_context.go`: Structures and logic for managing the context of a local workspace, potentially handling environment overrides, local state variables, and execution environment limits.
- `agents_md.go`: Logic for providing or injecting markdown-based context or instructions specific to the agents.

## Architectural Rules
- Context objects should be scoped appropriately (e.g., per-request, per-session, global).
- Avoid putting large unstructured data blobs into context; use structured types to enforce type safety.
- Context injection into LLM prompts should be modular to avoid blowing past token limits.
