# adapters/configsync/

Remote configuration synchronization adapter. Implements `ports.ConfigSyncPort`.

## Purpose

Fetches configuration updates from the API Gateway when the agent runs in **Super Duck**. Periodically syncs:

- Subagent capability profiles
- Security rules
- Remote policies

## Usage

Activated when `agent_mode = "super"` in `~/.duckops/config.toml`.

Sync interval: 60 seconds (background goroutine in bootstrap).
