# DuckOps Project Setup Guide

This guide will help you set up the complete DuckOps environment, including the infrastructure (RabbitMQ, Postgres), Backend Microservices (Server), and the Execution Engine (Agent).

---

## ⚡ Automated Setup (Recommended)

If you have Go and Docker installed, you can use these scripts to automate the environment setup:

### **Windows (PowerShell)**:

```powershell
./setup.ps1
```

### **Linux / macOS (Bash)**:

```bash
chmod +x setup.sh
./setup.sh
```

---

## 📋 Prerequisites

- **Go**: Version 1.24+
- **Docker**: Desktop or Engine with `docker-compose` support
- **LLM API Keys**: OpenAI, Anthropic, or OpenRouter keys (required for analysis)

---

## 🚀 Step 1: Clone and Repository Structure

DuckOps is built as a monorepo for development but supports independent component management. Ensure your directory looks like this:

```text
/duckops (Root)
├── agent/      # CLI & Worker Engine
├── server/     # Backend Services
├── shared/     # Common Types & Ports
├── .env        # Global Environment Variables
└── docker-compose.yml
```

---

## 🔑 Step 2: Environment Setup

1. Copy the `.env.example` (or create a new `.env`) in the root directory.
2. Update the credentials and API keys as follows:

```bash
# LLM Keys (Required)
OPENAI_API_KEY=sk-xxxx
OPENROUTER_API_KEY=sk-or-xxxx

# Infrastructure (Pre-filled for defaults)
RABBITMQ_URL=amqp://duckops:STRONG_PASSWORD@localhost:5672/
DATABASE_URL=postgres://duckops:STRONG_PASSWORD@localhost:5432/duckops?sslmode=disable
```

---

## 🏗️ Step 3: Start Infrastructure

DuckOps uses Docker to manage 3rd-party dependencies.

1. **Launch Postgres & RabbitMQ**:
   ```powershell
   docker compose up -d postgres rabbitmq
   ```
2. Verify connections:
   - **RabbitMQ Dashboard**: [localhost:15672](http://localhost:15672) (Login: `duckops` / `STRONG_PASSWORD`)
   - **Postgres**: Accessible on port `5432`.

---

## 🧠 Step 4: Build and Start Backend Services

You can build and run all microservices (`orchestrator`, `prompt-engine`, `result-processor`) using Docker:

```powershell
# Build and start all services
docker compose up -d --build
```

Alternatively, for **Local Development** (without Docker containerization for services):

1. Navigate to `/server`.
2. Run `go mod download`.
3. Run services individually:
   ```powershell
   go run ./services/orchestrator/cmd/orchestrator
   ```

---

## 🐚 Step 5: Initialize the Agent CLI

The Agent is your main interface for interacting with DuckOps.

1. **Build the CLI**:

   ```powershell
   cd agent
   go build -o duckops.exe ./cmd/agent-cli
   ```

2. **Run First-Time Setup**:
   ```powershell
   ./duckops.exe
   ```
   Follow the interactive prompts to:
   - Select your preferred LLM provider.
   - Add custom LLM providers if needed.
   - Verify the `config.yaml` is generated in `agent/`.

---

## ✅ Step 6: Verify the Installation

Trigger a simple test command to ensure the end-to-end flow works:

```powershell
./duckops.exe chat "Hello, are you ready to scan?"
```

- **Chat**: Communicates with the configured LLM through the agent.
- **Echo**: Tests the kernel execution flow.
- **Scan**: (Coming Soon) Triggers security tools.

---

## 🛠️ Troubleshooting

### "go: go.mod requires go >= 1.24"

Ensure you are using Go version 1.24.0 or higher. Run `go version` to check.

### "go.mod file not found" (In Docker Build)

Ensure you are running the `docker compose` command from the root folder (`/duckops`), as the context is set to `.` (root) to include the `shared` module.

### "No required module provides package..."

Run `go work sync` from the root directory to synchronize your local workspace across `agent`, `server`, and `shared`.
