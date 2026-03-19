package domain

import "github.com/SecDuckOps/shared/events"

// StreamEventType and StreamEvent are canonical in shared/events.
// These aliases keep existing agent code compiling without changes.

// StreamEventType is an alias to the canonical shared type.
type StreamEventType = events.StreamEventType

// StreamEvent is an alias to the canonical shared type.
type StreamEvent = events.StreamEvent

// Re-export constants for backward compatibility.
const (
	StreamEventStdout      = events.StreamEventStdout
	StreamEventStderr      = events.StreamEventStderr
	StreamEventThought     = events.StreamEventThought
	StreamEventStatus      = events.StreamEventStatus
	StreamEventWardenAlert = events.StreamEventWardenAlert
	StreamEventProgress    = events.StreamEventProgress
	StreamEventComplete    = events.StreamEventComplete
	StreamEventError       = events.StreamEventError
)

// NewStreamEvent creates a new stream event.
// Delegates to shared/events.
var NewStreamEvent = events.NewStreamEvent
