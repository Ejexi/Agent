#!/bin/bash

# DuckOps Setup Script (Bash/Linux/macOS)
# This script automates the installation of DuckOps.

set -e

echo "🦆 Starting DuckOps Setup..."

# 1. Check for Go 1.24+
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed."
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
if [[ "$(printf '%s\n' "1.24.0" "$GO_VERSION" | sort -V | head -n1)" != "1.24.0" ]]; then
    echo "❌ Error: Go 1.24.0 or higher is required. Found: $GO_VERSION"
    exit 1
fi

# 2. Check for Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Error: Docker is not installed."
    exit 1
fi

# 3. Setup Environment Variables
if [ ! -f .env ]; then
    echo "📝 Creating .env from defaults..."
    cat > .env <<EOL
# infrastructure
RABBITMQ_URL=amqp://duckops:STRONG_PASSWORD@localhost:5672/
DATABASE_URL=postgres://duckops:STRONG_PASSWORD@localhost:5432/duckops?sslmode=disable

# LLM Keys (REPLACE THEM!)
OPENAI_API_KEY=sk-xxxx
OPENROUTER_API_KEY=sk-or-xxxx

# Orchestrator
RABBITMQ_ORCH_IN_QUEUE=commands.accepted
RABBITMQ_RESULTS_QUEUE=results.processed
RABBITMQ_AGENT_TASKS_QUEUE=agent_tasks

# Prompt Engine
PROMPT_ENGINE_IN=raw_inputs

# Agent
AGENT_MODE=cloud
EOL
    echo "⚠️  Please edit your .env file with your actual LLM API keys before running the agent."
fi

# 4. Sync Workspace
echo "🔄 Syncing Go workspace..."
go work sync

# 5. Start Infrastructure (RabbitMQ & Postgres)
echo "🐳 Starting Infrastructure (RabbitMQ & Postgres)..."
docker compose up -d postgres rabbitmq

# 6. Build and Start Server Services
echo "🏗️  Building and starting Backend Services..."
docker compose up -d --build

# 7. Build Agent CLI
echo "🐚 Building Agent CLI..."
cd agent
go build -o duckops ./cmd/agent-cli
cd ..

echo "✅ Setup Complete!"
echo "------------------------------------------------"
echo "To start using DuckOps:"
echo "1. Configure your LLM keys in .env"
echo "2. Run: ./agent/duckops"
echo "------------------------------------------------"
