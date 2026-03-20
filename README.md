<div align="center">

# 🦆 DuckOps

**The Intelligent DevSecOps AI Agent**

[![Go Version](https://img.shields.io/github/go-mod/go-version/duckops/duckops)](https://golang.org/doc/devel/release.html)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Docker Support](https://img.shields.io/badge/docker-ready-blue.svg)](https://www.docker.com/)

*Scan your project for security issues, understand the risks, and get actionable fixes—all from your terminal.*
</div>

---

## 📖 Overview

DuckOps is an advanced, AI-driven DevSecOps agent that brings enterprise-grade security scanning directly to your terminal. It intelligently analyzes your codebase, orchestrates the right security scanners in isolated Docker containers, and uses LLM reasoning to filter noise and provide actionable remediation steps.

Whether you're developing a microservice in Go, a web app in Python, or managing infrastructure as code, DuckOps acts as your personal security engineer.

## ✨ Key Features

- **🧠 Intelligent Orchestration**: Automatically detects your project's tech stack and selects the most relevant scanners (e.g., skips `gosec` for a Python project).
- **🛡️ Comprehensive Scanning**: Supports SAST, SCA, Secrets detection, and IaC misconfigurations.
- **🐳 Secure Execution**: All scanners run within isolated Docker containers to protect your host environment.
- **💬 Conversational UI**: Ask questions about your code's security posture, investigate findings, and get fix recommendations via an interactive terminal UI (TUI).
- **📊 Noise Reduction**: Uses LLM capabilities to understand the context of vulnerabilities and prioritize what actually matters.
- **⚙️ CI/CD Ready**: Easy integration into automated pipelines with configurable failure thresholds and exit codes.

## 🚀 Quick Start

### Prerequisites
- Go 1.24 or higher
- Docker Engine running locally
- An API key for your preferred LLM provider (e.g., Anthropic, OpenAI)

### Installation

Build the binary from the source:

```bash
git clone https://github.com/SecDuckOps/agent.git
cd duckops
go build -o duckops ./cmd/duckops/...
```

Move the binary to your PATH (optional):
```bash
mv duckops /usr/local/bin/
```

### Usage Modes

DuckOps offers multiple interaction models to suit your workflow:

```bash
# 1. TUI Mode (Recommended)
# Launches the interactive Terminal User Interface
duckops

# 2. Conversational Scan REPL
# Focused, interactive scanning session
duckops --scan

# 3. Legacy LLM REPL
# Raw interactions with the underlying orchestrator
duckops --cli
```

## 🛠️ How It Works

1. **Context Gathering**: DuckOps analyzes your project structure to understand its components.
2. **Scanner Selection**: Determines which scanners (e.g., `semgrep`, `trivy`, `checkov`) are applicable natively.
3. **Execution**: Spawns isolated Docker containers to run the selected scanners securely.
4. **Analysis**: An Orchestrator AI processes the raw results, removes false positives, and assesses the actual risk based on your project's context.
5. **Reporting**: Surfaces prioritized findings and actionable fixes within a beautiful, markdown-rendered TUI.

*Example TUI Output:*
```text
> scan this project for vulnerabilities

🧠 Orchestrator Analysis
Go microservice with external API exposure.
Risk: Auth logic + crypto patterns detected — high risk zone.
Confidence: 89%

📊 Summary
| Severity | Count |
|----------|-------|
| 🔴 Critical| 2     |
| 🟠 High    | 8     |
```

## 🧰 Supported Scanners

| Category | Typical Scanners | Focus Area |
|----------|-----------------|------------|
| **SAST** | `semgrep`, `gosec`, `bandit` | Source code vulnerabilities, injection, logic flaws |
| **SCA** | `trivy`, `grype`, `osvscanner`| Known CVEs in third-party dependencies |
| **Secrets**| `gitleaks`, `trufflehog` | Hardcoded API keys, tokens, and credentials |
| **IaC** | `checkov`, `tfsec`, `kics` | Cloud misconfigurations and infrastructure flaws |

## ⚙️ Configuration

DuckOps works seamlessly out of the box, but you can customize its behavior via a configuration file located at `~/.duckops/config.toml`.

```toml
version = 1

[profile]
# Configure your LLM provider
provider = "anthropic"
model    = "claude-3-7-sonnet-latest"

[scan]
# Define the threshold for exiting with an error status (useful for CI/CD)
fail_on = "high"

[cloud]
# Enable to sync telemetry or reports to DuckOps Cloud
enabled = false
```

## 💻 Exit Codes

Designed with automation in mind, DuckOps returns deterministic exit codes:

| Code | Status | Description |
|:----:|:-------|:------------|
| `0` | **Success** | Clean run. No findings meet or exceed the `fail_on` threshold. |
| `1` | **Findings** | Vulnerabilities detected at or above the `fail_on` threshold. |
| `2` | **Env Error** | Docker daemon is unavailable or unreachable. |
| `3` | **Config Error**| Invalid configuration, missing API keys, or unsupported settings. |

## 📚 Documentation

Dive deeper into how DuckOps works and how to customize it:

- [🏗️ Architecture Overview](docs/ARCHITECTURE.md) — Explore the layer diagram, the AOrchestra pattern, and execution flow.
- [⌨️ TUI Guide](docs/TUI_GUIDE.md) — Learn about slash commands, `@` mentions, and keyboard shortcuts to master the interface.
- [🔒 Security Model](docs/SECURITY.md) — Understand our approach to container hardening, secrets scrubbing, and isolated execution.

---

<div align="center">
  <i>Built with ❤️ for secure software delivery.</i>
</div>
