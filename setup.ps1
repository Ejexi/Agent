# DuckOps Setup Script (PowerShell/Windows)
# This script automates the installation of DuckOps.

$ErrorActionPreference = "Stop"

Write-Host "🦆 Starting DuckOps Setup..." -ForegroundColor Cyan

# 1. Check for Go 1.24+
if (!(Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Error: Go is not installed." -ForegroundColor Red
    exit
}

$goVersion = (go version).Split(' ')[2].Replace("go", "")
if ($goVersion -lt "1.24.0") {
    Write-Host "❌ Error: Go 1.24.0 or higher is required. Found: $goVersion" -ForegroundColor Red
    exit
}

# 2. Check for Docker
if (!(Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Error: Docker is not installed." -ForegroundColor Red
    exit
}

# 3. Setup Environment Variables
if (!(Test-Path .env)) {
    Write-Host "📝 Creating .env from defaults..." -ForegroundColor Yellow
    $envContent = @"
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
"@
    Set-Content -Path .env -Value $envContent
    Write-Host "⚠️  Please edit your .env file with your actual LLM API keys before running the agent." -ForegroundColor Yellow
}

# 4. Sync Workspace
Write-Host "🔄 Syncing Go workspace..." -ForegroundColor Yellow
go work sync

# 5. Start Infrastructure (RabbitMQ & Postgres)
Write-Host "🐳 Starting Infrastructure (RabbitMQ & Postgres)..." -ForegroundColor Yellow
docker compose up -d postgres rabbitmq

# 6. Build and Start Server Services
Write-Host "🏗️  Building and starting Backend Services..." -ForegroundColor Yellow
docker compose up -d --build

# 7. Build Agent CLI
Write-Host "🐚 Building Agent CLI..." -ForegroundColor Yellow
Push-Location agent
go build -o duckops.exe ./cmd/agent-cli
Pop-Location

Write-Host "✅ Setup Complete!" -ForegroundColor Green
Write-Host "------------------------------------------------" -ForegroundColor White
Write-Host "To start using DuckOps:" -ForegroundColor White
Write-Host "1. Configure your LLM keys in .env" -ForegroundColor White
Write-Host "2. Run: ./agent/duckops.exe" -ForegroundColor White
Write-Host "------------------------------------------------" -ForegroundColor White
