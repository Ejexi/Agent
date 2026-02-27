# adapters/audit/

Audit logging adapter. Implements `ports.AuditPort`.

## Purpose

Records session audit entries for compliance and debugging. Supports local file logging and SSH-based remote backup.

## Configuration

Configured via `AuditConfig` in `~/.duckops/config.toml`:

```toml
[profiles.default.audit]
enabled = true
log_dir = "~/.duckops/audit"
backup_dir = "~/.duckops/audit/backups"
```
