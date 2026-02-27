# adapters/api/

HTTP API adapter for the DuckOps Agent.

## Purpose

Handles HTTP request/response serialization and routing. Exposes agent functionality via REST endpoints.

## Rules

- No business logic — delegates to ports/kernel.
- Handles only HTTP concerns (parsing, validation, response formatting).
