# adapters/server/

HTTP server adapter for the DuckOps Agent.

## Purpose

Runs the agent in server mode (`duckops serve`). Exposes REST/SSE/JSON-RPC endpoints for external clients to interact with the agent.

## Architecture Role

Server → Application Layer → Kernel → Tools

The server never contains business logic — it delegates to the Kernel via ports.
