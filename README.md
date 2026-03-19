# DuckOps

> DevSecOps AI Agent — scan your project for security issues, understand the risks, and get actionable fixes.

---

## Quick Start

```bash
# Prerequisites: Docker running, Go 1.24+
go build ./cmd/duckops/...

duckops          # TUI mode
duckops --scan   # conversational scan REPL
duckops --cli    # legacy LLM REPL
```

---

## What It Does

DuckOps runs security scanners in isolated Docker containers, uses an LLM to understand your project and plan which scanners matter, and surfaces findings in a clean terminal UI rendered via [glamour](https://github.com/charmbracelet/glamour).

```
> scan this project for vulnerabilities

🧠 Orchestrator Analysis
Go microservice with external API exposure.
Risk: Auth logic + crypto patterns detected — high risk zone.
Confidence: 89%

📊 Summary
| Severity | Count |
|----------|-------|
| 🔴 Critical | 2    |
| 🟠 High     | 8    |
```

---

## Configuration

Works without any setup. Optionally create `~/.duckops/config.toml`:

```toml
version = 1

[profile]
provider = "anthropic"
model    = "claude-sonnet-4-6"

[scan]
fail_on = "high"

[cloud]
enabled = false
```

---

## Scan Categories

| Category | Scanners | What it finds |
|----------|----------|---------------|
| SAST | semgrep, gosec, bandit | Code vulnerabilities, injection, auth flaws |
| SCA | trivy, grype, osvscanner | Known CVEs in dependencies |
| Secrets | gitleaks, trufflehog | Hardcoded API keys, tokens, passwords |
| IaC | checkov, tfsec, kics | Cloud misconfigurations |
| Deps | osvscanner, dependencycheck | Outdated packages with exploits |

Scanners are selected intelligently — a Python project won't run `gosec`.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Clean — no findings at/above `fail_on` |
| `1` | Findings found |
| `2` | Docker unavailable |
| `3` | Config error |

---

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — layer diagram, AOrchestra pattern
- [TUI Guide](docs/TUI_GUIDE.md) — slash commands, `@` mentions, keyboard shortcuts  
- [Security Model](docs/SECURITY.md) — container hardening, secrets scrubbing
