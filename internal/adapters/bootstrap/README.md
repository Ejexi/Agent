# adapters/bootstrap/

**Composition Root** — the single place where all dependencies are wired together.

## Files

| File           | Description                                                           |
| -------------- | --------------------------------------------------------------------- |
| `bootstrap.go` | `FromTOML()` — creates Kernel, LLM Registry, Tracker, registers tools |

## Key Function: `FromTOML()`

```
Config → Logger → LLM Registry → Kernel → Tracker → Register Tools → App
```

1. Loads the `default` profile from TOML config
2. Builds the LLM Registry (OpenAI, OpenRouter, Gemini)
3. Creates the Kernel with dependencies
4. Creates KernelBridge + Tracker (subagent system)
5. Registers all tools with the Kernel
6. Handles Super Duck (remote config sync)

## Rules

- This is the **only** place where concrete implementations are created.
- All dependency injection happens here.
- No business logic — only wiring.
