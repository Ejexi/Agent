# Kernel Logic (`agent/internal/kernel`)

## Purpose
The `kernel` directory implements the low-level processing engine for managing agent context, LLM interactions, dispatching tools, and executing core reasoning loops. It serves as the "brain" orchestrating the execution flow of prompts to actions.

## Files
- `kernel.go`: Central initialization and structuring of the AI kernel.
- `dispatcher.go`: Logic for routing tasks or outputs to the right handlers.
- `orchestrator.go`: Low-level control flow over multiple prompts and responses.
- `runtime.go`: The execution environment handling LLM lifecycle, token tracking, and safety guards.
- `context.go`: Maintains the specific runtime context provided to LLM sessions.

## Architectural Rules
- The Kernel wraps interactions with the LLM provider, providing a unified internal interface regardless of the specific AI backend.
- It interfaces with standard `domain` tools and actions but isolates the rest of the application from raw AI prompt management intricacies.
