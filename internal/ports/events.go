package ports

import (
	"context"
)

// EventBusPort defines the interface for the internal event system.
// Used for decoupling components within the same process.
type EventBusPort interface {
	// Publish sends an event to all subscribers of a topic.
	Publish(ctx context.Context, topic string, event interface{}) error

	// Subscribe returns a channel for receiving events from a topic.
	Subscribe(ctx context.Context, topic string) (<-chan interface{}, error)
}
