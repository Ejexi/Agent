# internal/

All internal packages for the DuckOps Agent. Not importable by external modules.

## Package Map

| Package                | Layer          | Description                                           |
| ---------------------- | -------------- | ----------------------------------------------------- |
| [domain/](domain/)     | Domain         | Core entities, interfaces, and contracts              |
| [kernel/](kernel/)     | Application    | Execution authority — Registry, Runtime, Dispatcher   |
| [ports/](ports/)       | Domain         | Interface definitions (Hexagonal Architecture ports)  |
| [config/](config/)     | Infrastructure | TOML configuration loading (`~/.duckops/config.toml`) |
| [adapters/](adapters/) | Infrastructure | Concrete implementations of ports                     |
| [tools/](tools/)       | Application    | Tool implementations registered with the Kernel       |

## Dependency Direction

```
adapters → ports → domain
tools    → ports + domain
kernel   → domain
config   → (standalone)
```

Domain and ports have **zero** infrastructure dependencies.
