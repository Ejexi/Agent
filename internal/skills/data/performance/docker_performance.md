---
name: docker-performance
description: Docker container performance — image layers, startup time, resource limits, scanner optimisation
---

# Docker Performance for Security Scanners

## Image Layer Optimisation

```dockerfile
# ❌ Each RUN = new layer
RUN apt-get update
RUN apt-get install -y curl
RUN apt-get clean

# ✅ Combine into one layer
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl && \
    rm -rf /var/lib/apt/lists/*

# ✅ Copy dependency files before source (cache busting)
COPY go.mod go.sum ./
RUN go mod download     # cached unless go.mod changes
COPY . .
RUN go build ...
```

## Scanner Image Warm-up Strategy

DuckOps `WarmupImages()` pulls all scanner images in parallel at startup:

```go
// Parallel pull — all images simultaneously
for _, name := range scannerNames {
    go func(n string) {
        _ = w.ensureImage(ctx, imageRef)
    }(name)
}
wg.Wait()
```

**Estimated pull times (first run):**
| Scanner | Image | Size |
|---------|-------|------|
| trivy | aquasec/trivy | ~100MB |
| semgrep | semgrep/semgrep | ~400MB |
| gitleaks | zricethezav/gitleaks | ~30MB |
| checkov | bridgecrew/checkov | ~200MB |

After first pull, `ensureImage` is just an `ImageInspect` call (~1ms).

## Container Startup Latency

| Phase | Typical time |
|-------|-------------|
| `ImageInspect` (cached) | <5ms |
| `ContainerCreate` | 50–200ms |
| `ContainerStart` | 10–50ms |
| Scanner execution | 5s–3min |
| Log capture + parse | <100ms |
| `ContainerRemove` | 50–100ms |

**Optimisation:** `PidsLimit: 100` reduces kernel overhead for fork-heavy scanners.

## Resource Limits in DuckOps

```go
Resources: container.Resources{
    Memory:    512 * 1024 * 1024,  // 512MB — prevents OOM thrashing
    CPUQuota:  50000,              // 50% of one core
    PidsLimit: &pidsLimit,         // 100 — prevents fork bombs
}
```

These limits also improve security — a compromised scanner cannot DoS the host.

## Parallel Scan Strategy

DuckOps runs categories in parallel, scanners within a category also in parallel (via `RunScanBatch`):

```
Category: SAST ──────────────────────────┐
  semgrep  ──►  findings                 │
  gosec    ──►  findings   (parallel)    │──► merge ──► tracker
  bandit   ──►  findings                 │
                                         │
Category: SCA ───────────────────────────┘
  trivy    ──►  findings
  grype    ──►  findings   (parallel)
```

Each category goroutine is independent — SAST and SCA run simultaneously.
