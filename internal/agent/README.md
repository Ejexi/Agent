# Core Agent Logic (`agent/internal/agent`)

## Purpose
The `agent` directory encapsulates the primary intelligent behaviors of the DuckOps system. It defines the core Agents (like `DuckOpsAgent` and `MasterAgent`), Orchestrator logic, and interaction protocols that coordinate various tools, subagents, and LLM providers to execute complex DevSecOps tasks.

## Files
- `duck_ops_agent.go`: Defines the primary conversational agent logic.
- `master_agent.go`: Logic for a coordinating agent that manages larger tasks and delegates to subagents or specific skills.
- `orchestrator.go` / `orchestrator_plan.go`: Handles the breakdown of complex goals into smaller manageable tasks and orchestrates their execution.
- `report_agent.go`: Specialized agent logic for generating comprehensive summaries or reports based on findings.
- `scan_task_tracker.go`: Tracks the progress and state of ongoing security scanning tasks.
- `intent.go` / `intent_test.go`: Logic for parsing and understanding user intent from natural language prompts.
- `types.go`: Core domain types specific to agent operations.
- `subagents/`: Directory containing specific, specialized subagent implementations.

## Architectural Rules
- This module is part of the core Domain/Application logic.
- It interfaces heavily with the `Kernel` for LLM capabilities, `Tools` for execution capabilities, and `Skills` for domain-specific knowledge.
- Agents should be stateless where possible or manage state cleanly via `CheckpointStore` patterns, allowing for session resumption and robust recovery.
