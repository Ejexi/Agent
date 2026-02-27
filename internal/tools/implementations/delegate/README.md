# tools/implementations/delegate/

Delegate tool — routes tasks to capability-matched sub-agents.

## Purpose

Accepts a task description and automatically selects the best-matching sub-agent capability profile from the `CapabilityRegistry`. More advanced than the raw `subagent` tool — it uses semantic matching.

## Registration

Registered in bootstrap as: `delegate.NewDelegateTool(tracker, registry)`
