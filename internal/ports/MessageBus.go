package ports

import (
	"context"

	"github.com/SecDuckOps/Agent/internal/domain"
)

// BusPort defines the interface for the message bus (RabbitMQ, NATS, etc.).
// Uses domain types — the adapter handles wire-format serialization internally.
type BusPort interface {
	// Publish sends a domain result to the specified topic.
	Publish(ctx context.Context, topic string, result domain.Result) error

	// PublishEvent sends any type of event to a specific topic (cloud bridge)
	PublishEvent(ctx context.Context, topic string, event interface{}) error

	// Subscribe registers a handler for incoming tasks on the specified topic.
	Subscribe(ctx context.Context, topic string, handler func(domain.Task)) error

	// Close gracefully shuts down the message bus connection.
	Close() error
}
