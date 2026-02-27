# cmd/

Entry points for the DuckOps Agent binary.

## Subdirectories

| Directory            | Description                             |
| -------------------- | --------------------------------------- |
| [duckops/](duckops/) | Main CLI application — `duckops` binary |

## Architecture Role

This is the **Entry Layer**. It depends only on `internal/kernel` and `internal/adapters/bootstrap`.

No business logic should live here — only CLI wiring and command definitions.
