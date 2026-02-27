# domain/subagent/

Domain types for the subagent lifecycle system.

## Purpose

Defines entities and value objects for subagent sessions, capabilities, and lifecycle states.

Used by the `ports.SubagentPort` and the `adapters/subagent/` package.

## Rules

- Pure domain types — no infrastructure imports.
- Defines the contract that the Tracker and SessionActor implement.
