package events

import (
	"context"
	"sync"

	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
)

// InMemoryEventBus is a simple implementation of EventBusPort for local development.
type InMemoryEventBus struct {
	logger shared_ports.Logger
	mu     sync.RWMutex
	subs   map[string][]chan interface{}
}

// NewInMemoryEventBus creates a new in-memory event bus.
func NewInMemoryEventBus(l shared_ports.Logger) *InMemoryEventBus {
	return &InMemoryEventBus{
		logger: l,
		subs:   make(map[string][]chan interface{}),
	}
}

// Publish emits an event to all subscribers.
func (b *InMemoryEventBus) Publish(ctx context.Context, topic string, event interface{}) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	b.logger.Debug(ctx, "Publishing event", shared_ports.Field{Key: "topic", Value: topic})

	if channels, ok := b.subs[topic]; ok {
		for _, ch := range channels {
			select {
			case ch <- event:
				// Successfully sent event
			default:
				// Subscriber buffer full, drop event to prevent blocking publisher
				b.logger.Info(ctx, "Event dropped: subscriber channel full", shared_ports.Field{Key: "topic", Value: topic})
			}
		}
	}
	return nil
}

// Subscribe registers a new subscriber for a topic.
func (b *InMemoryEventBus) Subscribe(ctx context.Context, topic string) (<-chan interface{}, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan interface{}, 100)
	b.subs[topic] = append(b.subs[topic], ch)
	return ch, nil
}

var _ ports.EventBusPort = (*InMemoryEventBus)(nil)
