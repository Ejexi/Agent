# domain/security/

Security domain types shared across the agent.

## Files

| File         | Description                                                                            |
| ------------ | -------------------------------------------------------------------------------------- |
| `warden.go`  | `NetworkRequest`, `NetworkPolicy`, `PolicyDecision`, `MTLSConfig` — Warden proxy types |
| `secrets.go` | `SecretMatch`, `PlaceholderMap` — secret detection and substitution types              |
| `audit.go`   | `AuditEntry`, `AuditSession` — session audit logging types                             |

## Purpose

Defines the security vocabulary of the system. Used by:

- `ports.WardenPort` — network sandbox
- `ports.SecretScannerPort` — secret detection
- `ports.AuditPort` — audit logging

## Rules

- Pure domain types — no infrastructure imports.
- These types flow through ports into adapters.
