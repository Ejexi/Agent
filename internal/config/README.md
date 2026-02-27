# config/

Configuration management for the DuckOps Agent.

## Files

| File                | Description                                                             |
| ------------------- | ----------------------------------------------------------------------- |
| `duckops_config.go` | TOML config structures, loading, and `~/.duckops/` directory management |

## Configuration Location

```
~/.duckops/
├── config.toml    # Main configuration file
└── data/
    └── local.db   # Local SQLite database
```

## Key Types

| Type            | Description                                                |
| --------------- | ---------------------------------------------------------- |
| `DuckOpsConfig` | Top-level config loaded from `config.toml`                 |
| `Profile`       | Named configuration profile (providers, security settings) |
| `Provider`      | LLM provider config (type, API key, model, base URL)       |
| `WardenConfig`  | Sandbox/isolation and mTLS settings                        |
| `SecretsConfig` | Secret substitution settings                               |
| `AuditConfig`   | Session audit logging settings                             |
| `Settings`      | Global settings (machine name, agent mode, server addr)    |

## Functions

| Function             | Description                                                         |
| -------------------- | ------------------------------------------------------------------- |
| `LoadTOML()`         | Loads config from `~/.duckops/config.toml`, auto-creates if missing |
| `EnsureDuckOpsDir()` | Creates `~/.duckops/` and subdirectories                            |
| `GetProfile(name)`   | Returns named profile, defaults to `"default"`                      |
| `DatabasePath()`     | Returns path to `~/.duckops/data/local.db`                          |
