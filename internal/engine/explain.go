package engine

import (
	"fmt"
	"strings"

	"github.com/SecDuckOps/agent/internal/telemetry"
)

// Explainer provides agent decision explainability.
type Explainer struct {
	tracer      *telemetry.Tracer
	auditLogger *telemetry.AuditLogger
}

// NewExplainer creates a new Explainer.
func NewExplainer(tracer *telemetry.Tracer, auditLogger *telemetry.AuditLogger) *Explainer {
	return &Explainer{
		tracer:      tracer,
		auditLogger: auditLogger,
	}
}

// ExplainTrace returns a human-readable trace summary for a given trace ID.
func (e *Explainer) ExplainTrace(traceID telemetry.TraceID) string {
	spans := e.tracer.GetTrace(traceID)
	if len(spans) == 0 {
		return fmt.Sprintf("No trace found for ID: %s", traceID)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Trace: %s (%d spans)\n", traceID, len(spans)))
	sb.WriteString(strings.Repeat("─", 60) + "\n")

	for _, span := range spans {
		duration := span.EndTime.Sub(span.StartTime)
		sb.WriteString(fmt.Sprintf("  [%s] %s (%v) — %s\n",
			span.SpanID, span.Operation, duration, span.Status))

		if len(span.Tags) > 0 {
			for k, v := range span.Tags {
				sb.WriteString(fmt.Sprintf("    tag: %s = %s\n", k, v))
			}
		}
	}

	return sb.String()
}

// ExplainDenials returns a summary of all recent policy denials.
func (e *Explainer) ExplainDenials() string {
	if e.auditLogger == nil {
		return "No audit logger configured"
	}

	// Use the AuditPolicyDeny constant from the security package
	entries := e.auditLogger.GetEntries()
	var denials []string
	for _, entry := range entries {
		if entry.Action == "policy.deny" {
			denials = append(denials, fmt.Sprintf("  [%s] Actor: %s → Target: %s (Policy: %v)",
				entry.Timestamp.Format("15:04:05"), entry.Actor, entry.Target, entry.Details))
		}
	}

	if len(denials) == 0 {
		return "✅ No policy denials recorded"
	}

	return fmt.Sprintf("🚨 %d policy denials:\n%s", len(denials), strings.Join(denials, "\n"))
}
