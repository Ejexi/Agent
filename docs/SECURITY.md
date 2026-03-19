# DuckOps — Security Model

## Container Isolation

Every scanner runs in a hardened Docker container. No scanner can affect the host filesystem or network.

### Hardening Configuration

| Setting | Value | Reason |
|---------|-------|--------|
| `CapDrop` | `ALL` | Remove all Linux capabilities |
| `SecurityOpt` | `no-new-privileges:true` | Prevent privilege escalation |
| `NetworkMode` | Configurable per scanner | Trivy/Semgrep need network for DB updates |
| `ReadonlyRootfs` | `false` | Scanners need `/tmp` write access |
| `Binds` | `hostPath:/scan/workspace:ro` | Host mount is read-only |
| `Memory` | 512 MB | Prevent OOM attacks |
| `CPUQuota` | 50% (50000) | Prevent CPU exhaustion |
| `PidsLimit` | 100 | Prevent fork bombs |
| `AutoRemove` | `false` | We manage removal via `defer` |

### Container Cleanup

Every `RunScan` call defers container removal:

```go
defer func() {
    cli.ContainerRemove(context.Background(), containerID,
        container.RemoveOptions{Force: true})
}()
```

This guarantees cleanup even on panic, timeout, or context cancellation.

---

## Secrets Protection

### In-Process Scrubbing

Before any user message is sent to the LLM, the `SecretScanner` scrubs it:

1. Known secret patterns (AWS keys, tokens, private keys) are replaced with `{{PLACEHOLDER_N}}`
2. After the LLM responds, placeholders are restored
3. The original secrets never leave the local process

### File Injection Limit

When a file is attached via `@mention`, it is:
- Limited to **100KB** per file
- Injected as a code block (content visible to LLM, but scrubbed first)
- Never uploaded to any external service unless cloud is explicitly configured

---

## Subagent Least-Privilege

Each scan subagent only has access to its own category's scanners:

```
sast:    semgrep, gosec, bandit, njsscan, brakeman
sca:     trivy, grype, osvscanner, dependencycheck
secrets: gitleaks, trufflehog, detectsecrets
iac:     checkov, tfsec, kics, terrascan, tflint
deps:    osvscanner, dependencycheck
```

A misconfigured SAST subagent cannot run a secrets scanner. The whitelist is enforced in `types.go:subagentScanners` and validated in `intelligent_base.go:selectScanners`.

---

## Cloud Push (Phase 4)

When cloud is configured, findings are pushed to the DuckOps API:

- **Source code is never included** in the cloud payload
- Only metadata is sent: CVE ID, severity, file path (relative), scanner name
- Push failures are non-fatal — local scan always succeeds independently
- API communication uses Bearer token auth over HTTPS

---

## Error Codes

All errors use structured `AppError` with codes:

| Code | Meaning |
|------|---------|
| `ERR_DUCKOPS_1000` | Internal error |
| `ERR_DUCKOPS_1001` | Not found |
| `ERR_DUCKOPS_1002` | Invalid input |
| `ERR_DUCKOPS_2001` | Agent failed |
| `ERR_DUCKOPS_3000` | Tool not found |
| `ERR_DUCKOPS_3001` | Tool execution failed |
| `ERR_DUCKOPS_4000` | Auth failed |
| `ERR_DUCKOPS_4003` | Permission denied |
| `ERR_DUCKOPS_6000` | Security violation |
| `ERR_DUCKOPS_6001` | Execution failed |

---

## Subagent Depth Limits

To prevent runaway recursive subagent spawning:

```
Maximum subagent depth: 3
```

This is enforced in `adapters/subagent/tracker.go`. A subagent spawned at depth 3 cannot spawn further subagents.
