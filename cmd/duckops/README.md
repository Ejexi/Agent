# cmd/duckops/

Main CLI application for the DuckOps Agent.

## Files

| File            | Description                                        |
| --------------- | -------------------------------------------------- |
| `main.go`       | Application entry point                            |
| `root.go`       | Root Cobra command, global flags, bootstrap wiring |
| `run.go`        | `duckops run` — interactive agent session          |
| `serve.go`      | `duckops serve` — HTTP/API server mode             |
| `runtime.go`    | Shared runtime setup (Kernel + Tracker init)       |
| `login.go`      | `duckops login` — API Gateway authentication       |
| `config_cmd.go` | `duckops config` — view/edit configuration         |

## Execution Flow

```
main.go → root.go → bootstrap.FromTOML() → Kernel + Tracker
                   → run.go / serve.go (chosen subcommand)
```

## Rules

- Must only import `internal/kernel`, `internal/adapters/bootstrap`, and `internal/config`.
- No business logic — only CLI glue.
