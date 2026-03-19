package events

import (
	"context"
	"sync"

	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
)

// subscription tracks a single subscriber with its ID for cleanup.
type subscription struct {
	id uint64
	ch chan interface{}
}

// InMemoryEventBus is a simple implementation of EventBusPort for local development.
type InMemoryEventBus struct {
	logger  shared_ports.Logger
	mu      sync.RWMutex
	subs    map[string][]subscription
	counter uint64
}

// NewInMemoryEventBus creates a new in-memory event bus.
func NewInMemoryEventBus(l shared_ports.Logger) *InMemoryEventBus {
	return &InMemoryEventBus{
		logger: l,
		subs:   make(map[string][]subscription),
	}
}

// Publish emits an event to all subscribers on a topic.
func (b *InMemoryEventBus) Publish(ctx context.Context, topic string, event interface{}) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.logger != nil {
		b.logger.Debug(ctx, "Publishing event", shared_ports.Field{Key: "topic", Value: topic})
	}

	for _, sub := range b.subs[topic] {
		select {
		case sub.ch <- event:
		default:
			// Subscriber buffer full — drop event to avoid blocking publisher
			if b.logger != nil {
				b.logger.Info(ctx, "Event dropped: subscriber channel full",
					shared_ports.Field{Key: "topic", Value: topic},
					shared_ports.Field{Key: "sub_id", Value: sub.id},
				)
			}
		}
	}
	return nil
}

// Subscribe registers a new subscriber for a topic.
// Returns the channel and a unique subscription ID for later unsubscription.
func (b *InMemoryEventBus) Subscribe(ctx context.Context, topic string) (<-chan interface{}, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.counter++
	ch := make(chan interface{}, 100)
	b.subs[topic] = append(b.subs[topic], subscription{id: b.counter, ch: ch})
	return ch, nil
}

// Unsubscribe removes a subscriber by its channel reference and closes the channel.
// Callers should call this when done consuming events to prevent memory leaks.
func (b *InMemoryEventBus) Unsubscribe(ctx context.Context, topic string, ch <-chan interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subs[topic]
	for i, sub := range subs {
		if sub.ch == ch {
			// Close the channel so any range loop on it exits cleanly
			close(sub.ch)
			b.subs[topic] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// Close shuts down the event bus and closes all subscriber channels.
func (b *InMemoryEventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for topic, subs := range b.subs {
		for _, sub := range subs {
			close(sub.ch)
		}
		delete(b.subs, topic)
	}
}

var _ ports.EventBusPort = (*InMemoryEventBus)(nil)
