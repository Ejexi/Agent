package telemetry

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TraceID uniquely identifies a distributed trace.
type TraceID string

// SpanID uniquely identifies a span within a trace.
type SpanID string

// Span represents a single unit of work within a trace.
type Span struct {
	TraceID   TraceID                `json:"trace_id"`
	SpanID    SpanID                 `json:"span_id"`
	ParentID  SpanID                 `json:"parent_id,omitempty"`
	Operation string                 `json:"operation"` // e.g., "kernel.execute", "warden.evaluate"
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time,omitempty"`
	Tags      map[string]string      `json:"tags,omitempty"`
	Status    string                 `json:"status"` // "ok", "error"
	Details   map[string]interface{} `json:"details,omitempty"`
}

// Tracer provides distributed tracing capabilities.
type Tracer struct {
	mu    sync.Mutex
	spans []Span
}

// NewTracer creates a new distributed tracer.
func NewTracer() *Tracer {
	return &Tracer{
		spans: make([]Span, 0),
	}
}

// StartSpan begins a new trace span.
func (t *Tracer) StartSpan(_ context.Context, traceID TraceID, operation string, parentID SpanID) *Span {
	span := &Span{
		TraceID:   traceID,
		SpanID:    SpanID(fmt.Sprintf("span_%d", time.Now().UnixNano())),
		ParentID:  parentID,
		Operation: operation,
		StartTime: time.Now(),
		Tags:      make(map[string]string),
		Details:   make(map[string]interface{}),
		Status:    "ok",
	}
	return span
}

// EndSpan completes a span and records it.
func (t *Tracer) EndSpan(span *Span) {
	span.EndTime = time.Now()
	t.mu.Lock()
	t.spans = append(t.spans, *span)
	t.mu.Unlock()
}

// GetTrace returns all spans for a given trace ID.
func (t *Tracer) GetTrace(traceID TraceID) []Span {
	t.mu.Lock()
	defer t.mu.Unlock()

	var result []Span
	for _, s := range t.spans {
		if s.TraceID == traceID {
			result = append(result, s)
		}
	}
	return result
}
