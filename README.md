<div align="center">

# 🦆 DuckOps Agent

**The Intelligent DevSecOps AI Agent**

[![Go Version](https://img.shields.io/github/go-mod/go-version/duckops/duckops)](https://golang.org/doc/devel/release.html)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Docker Support](https://img.shields.io/badge/docker-ready-blue.svg)](https://www.docker.com/)
[![CI Status](https://img.shields.io/github/actions/workflow/status/SecDuckOps/agent/ci.yml?branch=main)](https://github.com/SecDuckOps/agent/actions)

*Scan your project for security issues, understand the risks, and get actionable fixes—all from your terminal.*

[Installation](#-quick-start) •
[Features](#-key-features) •
[Documentation](#-documentation) •
[Supported Scanners](#-supported-scanners) •
[Contributing](#-contributing)
</div>

---

## 📖 Overview

**DuckOps** is a next-generation, AI-driven DevSecOps agent that brings enterprise-grade security scanning directly to your command line. Built on the [AOrchestra](docs/ARCHITECTURE.md) pattern, it intelligently analyzes your codebase, orchestrates the right security scanners in isolated Docker containers, and uses LLM-powered context awareness to filter out false positives.

Whether you're writing a highly concurrent Go microservice, a complex React frontend, or Terraform infrastructure, DuckOps acts as your tireless, personal security engineer.

## ❓ Why DuckOps?

Traditional security scanners output thousands of uncontextualized alerts. Developers suffer from alert fatigue and spend hours triaging "Critical" vulnerabilities that are completely unreachable in practice. 

DuckOps solves this by:
1. **Understanding Context**: It reads your code structure to see *how* libraries are used.
2. **Eliminating Noise**: It cross-references scanner outputs with actual project context.
3. **Explaining Clearly**: It explains *why* something is a risk in plain English.
4. **Providing Fixes**: It generates patch suggestions tailored to your codebase.

## ✨ Key Features

- **🧠 Intelligent Orchestration**: Automatically detects your project's tech stack (Go, Python, JS, IaC) and selects only the relevant tools.
- **🛡️ Comprehensive Scanning Pipeline**: Seamlessly runs SAST, SCA, Secrets Detection, and IaC linting in parallel.
- **🐳 Secure, Ephemeral Execution**: Scanners run inside locked-down, ephemeral Docker containers. Your host machine stays clean and secure.
- **💬 Immersive TUI**: An interactive, beautiful Terminal User Interface powered by Charmbracelet. Chat with your security findings, ask for code fixes, or explore vulnerabilities.
- **📊 AI Noise Reduction**: Uses frontier LLMs (Claude, GPT-4) to reason about vulnerabilities, suppressing irrelevant CVEs and highlighting true risks.
- **⚙️ CI/CD Native**: Designed to break the build when it matters. Configurable failure thresholds mean it fits perfectly in GitHub Actions, GitLab CI, or Jenkins.

## 🚀 Quick Start

### Prerequisites
- **Go** 1.24+ (for building from source)
- **Docker Engine** running locally
- **API Key** for your preferred LLM provider (Anthropic, OpenAI, etc.)

### Installation

Clone the repository and compile the binary:

```bash
git clone https://github.com/SecDuckOps/agent.git
cd agent
go build -o duckops ./cmd/duckops/...
```

Move the binary to your executable path:
```bash
sudo mv duckops /usr/local/bin/
```

### Usage Modes

DuckOps adapts to how you work:

```bash
# 1. TUI Mode (Recommended)
# Launches a rich, interactive console
duckops

# 2. Conversational Scan REPL
# A fast CLI chat interface for deep dives into a specific directory
duckops --scan

# 3. Legacy Automated Run
# Ideal for piping or scripts
duckops --cli
```

## 🖥️ The Interactive TUI

In TUI mode, DuckOps provides a rich interface. 

### Useful Slash Commands:
- `/scan` - Triggers a comprehensive background scan of the current directory.
- `/fix [vuln-id]` - Asks the AI to generate a patch for a specific vulnerability.
- `/clear` - Clears the current terminal session history.
- `/help` - Shows the help menu and keyboard shortcuts.

*Example TUI Output:*
```text
> /scan

🧠 Analyzing Project...
Found: Go backend (Go 1.24), Dockerfile, GitHub Workflows.
Orchestrating scanners: gosec, trivy-go, trivy-docker, gitleaks.

📊 Summary
| Severity | Count | Fixable |
|----------|-------|---------|
| 🔴 Critical| 1     | Yes     |
| 🟠 High    | 3     | Yes     |
| 🟡 Medium  | 12    | No      |

🛑 Risk: Hardcoded JWT secret found in `internal/auth/keys.go`. 
💡 Suggestion: Use `os.Getenv("JWT_SECRET")` instead. Type `/fix SEC-001` to apply.
```

## 🛠️ How It Works (The Pipeline)

1. **Context Gathering**: The local agent crawls the directory, identifying languages, frameworks, and dependency files.
2. **Scanner Selection**: The LLM Orchestrator dynamically generates a plan of which Dockerized tools to run.
3. **Execution**: The local agent pulls (if necessary) and runs the scanner images against read-only mounts of your code.
4. **Aggregation**: Outputs are converted into a standardized JSON format.
5. **Contextual Analysis**: The AI reviews the findings, correlates them with your code, and filters out false positives.
6. **Reporting**: Results are streamed to the TUI or outputted as an artifact (JSON/Sarif).

## 🧰 Supported Scanners

DuckOps acts as a unifying layer over the best open-source security tools:

| Category | Supported Tools | Focus Area |
|----------|-----------------|------------|
| **SAST** | `semgrep`, `gosec`, `bandit`, `njsscan` | Source code vulnerabilities, injection, logic flaws |
| **SCA** | `trivy`, `grype`, `osvscanner` | Known CVEs in third-party packages |
| **Secrets**| `gitleaks`, `trufflehog` | Hardcoded API keys, tokens, and credentials |
| **IaC** | `checkov`, `tfsec`, `kics` | Cloud misconfigurations and Terraform flaws |

## ⚙️ Configuration

Store your configuration in `~/.duckops/config.toml` to customize LLM providers, rules, and telemetry.

```toml
version = 1

[profile]
provider = "anthropic"           # Supported: anthropic, openai, local (ollama)
model    = "claude-3-7-sonnet-latest" 
api_key_env = "ANTHROPIC_API_KEY" # Environment variable to read key from

[scan]
fail_on = "high"                 # Exit code 1 if findings >= high
ignore_dirs = ["node_modules", "vendor", ".git"]

[docker]
auto_pull = true                 # Automatically pull missing scanner images

[cloud]
enabled = false                  # Disable telemetry by default
```

## 🏗️ Continuous Integration (CI/CD)

Integrate DuckOps into your GitHub Actions workflow:

```yaml
name: DuckOps Security Scan
on: [push, pull_request]

jobs:
  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run DuckOps Agent
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        run: |
          curl -sL https://github.com/SecDuckOps/agent/releases/latest/download/duckops-linux-amd64 -o duckops
          chmod +x duckops
          ./duckops --cli
```

## 💻 Exit Codes

DuckOps provides deterministic exit codes for CI environments:

| Code | Status | Description |
|:----:|:-------|:------------|
| `0` | **Success** | Clean run. No findings meet the `fail_on` threshold. |
| `1` | **Findings** | Vulnerabilities detected at or above threshold. |
| `2` | **Env Error** | Docker daemon is unavailable, offline, or missing permissions. |
| `3` | **Config Error**| Invalid configuration, missing API keys, or unresolvable models. |

## 📚 Documentation

Dive deeper into extending and configuring DuckOps:

- [🏗️ Architecture & AOrchestra](docs/ARCHITECTURE.md)
- [⌨️ TUI Interaction Guide](docs/TUI_GUIDE.md)
- [🔒 Security & Threat Model](docs/SECURITY.md)
- [🔌 Writing Custom Scanners](docs/SCANNERS.md) (coming soon)

## 🤝 Contributing

We welcome contributions! Whether it's adding a new scanner definition, improving the LLM prompts, or fixing a UI bug.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.

---

<div align="center">
  <i>Built with ❤️ for secure software delivery.</i>
</div>
