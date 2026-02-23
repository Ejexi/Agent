package ports

import (
	"context"
	"duckops/internal/domain"
)

// BusPort defines the interface for the message bus (RabbitMQ, NATS, etc.).
// Uses domain types — the adapter handles wire-format serialization internally.
type BusPort interface {
	// Publish sends a domain result to the specified topic.
	// The adapter is responsible for serializing the result to the wire format.
	Publish(ctx context.Context, topic string, result domain.Result) error

	// Subscribe registers a handler for incoming tasks on the specified topic.
	// The adapter is responsible for deserializing the wire format into domain.Task.
	Subscribe(ctx context.Context, topic string, handler func(domain.Task)) error

	// Close gracefully shuts down the message bus connection.
	Close() error
}
