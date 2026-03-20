# Telemetry & Observability (`agent/internal/telemetry`)

## Purpose
The `telemetry` directory handles structural logging, distributed tracing, and specialized auditing functions for the DuckOps agent. It ensures the system's actions are inspectable, measurable, and compliant with audit requirements.

## Files
- `audit.go`: Contains logic for structured audit logging, particularly crucial for capturing sensitive operations, tool executions, and state changes for security review.
- `tracing.go`: Implements distributed or local span tracing, allowing developers to contextualize the latency and execution paths of complex agent reasoning loops.

## Architectural Rules
- **Cross-Cutting Concern:** Telemetry is injected into various parts of the application as a support mechanism, typically through interfaces defined in `ports`.
- **Performance Aware:** Telemetry recording should be non-blocking and minimally impact the core reasoning loop performance.
