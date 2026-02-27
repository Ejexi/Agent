# tools/

Tool system for the DuckOps Agent.

Tools are the execution units of the agent. They implement the `domain.Tool` interface and are registered with the Kernel at startup.

## Subdirectories

| Directory                            | Description                                    |
| ------------------------------------ | ---------------------------------------------- |
| [base/](base/)                       | Base abstractions for building type-safe tools |
| [implementations/](implementations/) | Concrete tool implementations                  |

## Tool Contract

All tools must:

- Be **stateless**
- Receive a `Task` (via `ExecuteRaw`)
- Return a `Result`
- Use **ports only** for external access

Tools must **not**:

- Contain infrastructure logic
- Contain orchestration logic
- Access databases, message queues, or filesystems directly

## Interface

```go
type Tool interface {
    Name() string
    Schema() ToolSchema
    ExecuteRaw(ctx context.Context, input map[string]interface{}) (Result, error)
}
```
