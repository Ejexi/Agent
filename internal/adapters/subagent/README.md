# adapters/subagent/

Subagent lifecycle management system. Implements `ports.SessionManager`.

## Files

| File                     | Description                                                                    |
| ------------------------ | ------------------------------------------------------------------------------ |
| `tracker.go`             | `Tracker` — manages all subagent sessions, implements `SessionManager`         |
| `session.go`             | `SessionActor` — individual subagent session with LLM conversation loop        |
| `bridge.go`              | `KernelBridge` — connects subagent system to Kernel (tool execution + schemas) |
| `capability_registry.go` | `CapabilityRegistry` — manages capability profiles for delegate tool           |
| `eventlog.go`            | `EventLog` — structured event logging per session                              |
| `eventlog_test.go`       | Unit tests for `EventLog`                                                      |

## Architecture

```
Tracker (SessionManager)
   ├── SessionActor (per subagent)
   │      └── KernelBridge → Kernel.Execute()
   └── CapabilityRegistry (for delegation)
```

The subagent system is **decoupled** from the Kernel:

- Kernel executes tools (execution authority)
- Tracker manages session lifecycles (orchestration)
- KernelBridge adapts between the two

## Rules

- Subagent logic does NOT live in the Kernel.
- All tool execution goes through `KernelBridge → Kernel.Execute()`.
