# Application Ports (`agent/internal/ports`)

## Purpose
The `ports` directory defines the interfaces for interacting with the outside world, adhering to Hexagonal Architecture. It cleanly separates the 'what' (the interface) from the 'how' (the adapter implementation).

## Files
- Interface definitions spanning multiple domain capabilities: `application.go` (Use cases), `agent_hook.go`, `audit.go`, `events.go`, `taskengine.go`, `warden.go` (Security checks), `mcp.go`, `session_manager.go`, etc.

## Architectural Rules
- **Interfaces Only:** This directory should contain Go `interface` definitions and potentially lightweight DTOs, but ZERO concrete implementation logic.
- **Domain Centric:** Interfaces are defined based on what the application needs, not based on what a specific technology provides. Adapters implement these ports.
