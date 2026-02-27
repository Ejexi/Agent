# tools/implementations/scan/

Scan tool — security scanning across multiple scanner types.

## Purpose

Triggers security scans (SAST, DAST, Secrets, Container, Dependency, IaC) and returns structured `ScanResult` with vulnerabilities, severity counts, and remediation suggestions.

Uses LLM for intelligent scan analysis and `MemoryPort` for result storage.

## Supported Scanner Types

| Type         | Description                          |
| ------------ | ------------------------------------ |
| `SAST`       | Static Application Security Testing  |
| `DAST`       | Dynamic Application Security Testing |
| `SECRETS`    | Secret/credential detection          |
| `CONTAINER`  | Container image scanning             |
| `DEPENDENCY` | Dependency vulnerability scanning    |
| `IAC`        | Infrastructure as Code scanning      |

## Registration

Registered in bootstrap as: `scan.NewScanTool(deps.LLM, deps.Memory)`
