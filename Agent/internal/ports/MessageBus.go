package ports

import (
	"agent/internal/domain"
	"context"
)

// BusPort defines the interface for the message bus.
type BusPort interface {
	Publish(ctx context.Context, topic string, message interface{}) error
	Subscribe(ctx context.Context, topic string, handler func(domain.Task)) error
}
