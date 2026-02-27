# tools/base/

Base abstractions for building type-safe tools.

## Purpose

Provides `TypedToolBase[P]` — a generic base struct that simplifies tool creation by handling parameter parsing automatically. New tools should embed this base to get type-safe parameter handling with minimal boilerplate.

## Usage

```go
type MyParams struct {
    Name string `json:"name"`
}

type MyTool struct {
    base.TypedToolBase[MyParams]
}
```
