# tools/implementations/

Concrete tool implementations registered with the Kernel.

## Available Tools

| Directory                  | Tool Name            | Description                                              |
| -------------------------- | -------------------- | -------------------------------------------------------- |
| [chat/](chat/)             | `chat`               | LLM conversation via function calling                    |
| [delegate/](delegate/)     | `delegate`           | Delegates tasks to capability-matched sub-agents         |
| [scan/](scan/)             | `scan`               | Security scanning (SAST, DAST, Secrets, Container, etc.) |
| [subagent/](subagent/)     | `subagent`, `resume` | Spawn and resume sub-agent sessions                      |


## Adding a New Tool

1. Create a new directory: `implementations/<tool_name>/`
2. Implement `domain.Tool` interface (or embed `base.TypedToolBase[P]`)
3. Register in `adapters/bootstrap/bootstrap.go` → `registerTools()`
4. Add a `README.md` to the new directory
