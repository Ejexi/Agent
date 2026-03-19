# DuckOps Configuration Reference

Config file location: `~/.duckops/config.toml`

DuckOps works with zero configuration. This file is optional — all settings have sensible defaults.

---

## Full Example

```toml
version = 1

# ── LLM Provider ───────────────────────────────────────────────────
[profile]
provider = "anthropic"                  # anthropic | openai | gemini | ollama
model    = "claude-sonnet-4-6"
# api_key = ""                          # or set ANTHROPIC_API_KEY env var

# ── Scan Settings ──────────────────────────────────────────────────
[scan]
fail_on = "high"                        # exit(1) when findings at this level or above
format  = "text"                        # text | json

[scan.categories]
sast    = true
sca     = true
secrets = true
iac     = true
deps    = true
dast    = false                         # future — requires live target URL

# ── Scanner Overrides ──────────────────────────────────────────────
# Override which scanners each category uses.
# Default: determined by intelligent subagent based on project signals.
[scanners]
sast    = ["semgrep", "gosec"]          # override for Go-only projects
sca     = ["trivy", "grype"]
secrets = ["gitleaks", "trufflehog", "detectsecrets"]
iac     = ["checkov", "tfsec"]
deps    = ["osvscanner"]

# ── Server (HTTP/SSE API) ──────────────────────────────────────────
[settings]
server_addr = ":8090"                   # local API server address

# ── Cloud Push (Phase 4) ───────────────────────────────────────────
[cloud]
enabled = false
api_url = "https://api.duckops.io"
api_key = ""                            # or set DUCKOPS_API_KEY env var
org_id  = ""
```

---

## Environment Variables

All sensitive values can be passed as environment variables instead of config file:

| Variable | Overrides |
|----------|-----------|
| `ANTHROPIC_API_KEY` | `profile.api_key` for Anthropic |
| `OPENAI_API_KEY` | `profile.api_key` for OpenAI |
| `GEMINI_API_KEY` | `profile.api_key` for Gemini |
| `DUCKOPS_API_KEY` | `cloud.api_key` |
| `DUCKOPS_ORG_ID` | `cloud.org_id` |

---

## `[profile]` — LLM Provider

| Key | Default | Description |
|-----|---------|-------------|
| `provider` | first configured | LLM provider name |
| `model` | provider default | Model identifier |

**Provider values:**

| Provider | Example Model |
|----------|--------------|
| `anthropic` | `claude-sonnet-4-6` |
| `openai` | `gpt-4o` |
| `gemini` | `gemini-1.5-pro` |
| `ollama` | `llama3.2` |
| `openrouter` | `anthropic/claude-3-5-sonnet` |

---

## `[scan]` — Scan Behaviour

| Key | Default | Description |
|-----|---------|-------------|
| `fail_on` | `"high"` | Severity threshold for exit code 1. Values: `critical`, `high`, `medium`, `low`, `""` (never fail) |
| `format` | `"text"` | Output format: `text` (glamour-rendered) or `json` |

---

## `[scan.categories]` — Enable/Disable Categories

| Key | Default | Description |
|-----|---------|-------------|
| `sast` | `true` | Static analysis |
| `sca` | `true` | Software composition analysis |
| `secrets` | `true` | Secret detection |
| `iac` | `true` | Infrastructure-as-code |
| `deps` | `true` | Dependency audit |
| `dast` | `false` | Dynamic analysis (future) |

---

## `[scanners]` — Scanner Overrides

When set, these override the intelligent subagent's scanner selection. Useful when:
- You want deterministic scanner selection in CI
- You know exactly which scanners apply to your project
- You want to reduce scan time by limiting scanners

If not set (default), each subagent uses the LLM to select the most relevant scanners based on detected project signals.

---

## `[settings]` — Runtime

| Key | Default | Description |
|-----|---------|-------------|
| `server_addr` | `":8090"` | HTTP/SSE API listen address |

---

## `[cloud]` — Cloud Push

| Key | Default | Description |
|-----|---------|-------------|
| `enabled` | `false` | Enable cloud push after scan |
| `api_url` | `""` | DuckOps cloud API endpoint |
| `api_key` | `""` | API authentication key |
| `org_id` | `""` | Organisation identifier |

Cloud push is non-fatal — if it fails, the local scan result is unaffected.

**What is sent to the cloud:**
- Finding ID, severity, scanner name
- Relative file path and line number
- CVE ID (if present)
- Scan timestamp

**What is NEVER sent:**
- Source code content
- Environment variables
- Secrets or credentials
- Absolute file paths
