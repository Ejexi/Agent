package domain

import "time"

// StreamEventType classifies structured output events for the TUI.
type StreamEventType string

const (
	StreamEventStdout      StreamEventType = "stdout"
	StreamEventStderr      StreamEventType = "stderr"
	StreamEventThought     StreamEventType = "thought"
	StreamEventStatus      StreamEventType = "status"
	StreamEventWardenAlert StreamEventType = "warden_alert"
	StreamEventProgress    StreamEventType = "progress"
	StreamEventComplete    StreamEventType = "complete"
	StreamEventError       StreamEventType = "error"
)

// StreamEvent is a structured output event for progressive rendering in the TUI.
type StreamEvent struct {
	TaskID    string          `json:"task_id"`
	EventType StreamEventType `json:"event_type"`
	Payload   []byte          `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// NewStreamEvent creates a new stream event.
func NewStreamEvent(taskID string, eventType StreamEventType, payload []byte) StreamEvent {
	return StreamEvent{
		TaskID:    taskID,
		EventType: eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	}
}
