package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/SecDuckOps/Agent/internal/domain"
	"github.com/SecDuckOps/Shared/types"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Adapter implements ports.BusPort using RabbitMQ (AMQP 0.9.1).
// It contains zero business logic — only infrastructure concerns.
// Serialization/deserialization happens here (adapter responsibility).
type Adapter struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex
}

// Config holds connection parameters for the RabbitMQ adapter.
type Config struct {
	URL      string // Optional full AMQP connection string
	Host     string
	Port     int
	User     string
	Password string
	VHost    string
}

// DSN builds an AMQP connection string from the config.
func (c Config) DSN() string {
	if c.URL != "" {
		return c.URL
	}
	if c.VHost == "" {
		c.VHost = "/"
	}
	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		c.User, c.Password, c.Host, c.Port, c.VHost,
	)
}

// NewAdapter creates a new RabbitMQ adapter and establishes a connection.
func NewAdapter(cfg Config) (*Adapter, error) {
	conn, err := amqp.Dial(cfg.DSN())
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "rabbitmq: failed to connect")
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, types.Wrap(err, types.ErrCodeInternal, "rabbitmq: failed to open channel")
	}

	return &Adapter{
		conn:    conn,
		channel: ch,
	}, nil
}

// PublishEvent serializes any event to JSON and sends it.
func (a *Adapter) PublishEvent(ctx context.Context, topic string, event interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "rabbitmq: failed to marshal event")
	}

	_, err = a.channel.QueueDeclare(
		topic, true, false, false, false, nil,
	)
	if err != nil {
		return types.Wrapf(err, types.ErrCodeInternal, "rabbitmq: failed to declare queue %q", topic)
	}

	return a.channel.PublishWithContext(ctx,
		"", topic, false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         data,
		},
	)
}

func (a *Adapter) Publish(ctx context.Context, topic string, result domain.Result) error {
	return a.PublishEvent(ctx, topic, result)
}

// Subscribe registers a handler that is invoked for each message on the given topic.
// The adapter deserializes raw AMQP bytes into domain.Task before passing to the handler.
func (a *Adapter) Subscribe(ctx context.Context, topic string, handler func(domain.Task)) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Ensure the queue exists (idempotent)
	_, err := a.channel.QueueDeclare(
		topic, // queue name
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return types.Wrapf(err, types.ErrCodeInternal, "rabbitmq: failed to declare queue %q", topic)
	}

	msgs, err := a.channel.Consume(
		topic, // queue
		"",    // consumer tag (auto-generated)
		false, // auto-ack (manual ack for reliability)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return types.Wrapf(err, types.ErrCodeInternal, "rabbitmq: failed to start consumer on %q", topic)
	}

	// Dispatch incoming messages in a background goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("rabbitmq: consumer for %q shutting down: %v", topic, ctx.Err())
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Printf("rabbitmq: channel closed for %q", topic)
					return
				}

				// Adapter handles deserialization
				var task domain.Task
				if err := json.Unmarshal(msg.Body, &task); err != nil {
					log.Printf("rabbitmq: failed to unmarshal task on %q: %v", topic, err)
					msg.Nack(false, false) // Reject malformed messages
					continue
				}

				handler(task)

				// Acknowledge successful processing
				if ackErr := msg.Ack(false); ackErr != nil {
					log.Printf("rabbitmq: failed to ack message on %q: %v", topic, ackErr)
				}
			}
		}
	}()

	return nil
}

// Close gracefully shuts down the channel and connection.
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var errs []error
	if a.channel != nil {
		if err := a.channel.Close(); err != nil {
			errs = append(errs, types.Wrap(err, types.ErrCodeInternal, "rabbitmq: channel close"))
		}
	}
	if a.conn != nil {
		if err := a.conn.Close(); err != nil {
			errs = append(errs, types.Wrap(err, types.ErrCodeInternal, "rabbitmq: connection close"))
		}
	}

	if len(errs) > 0 {
		return types.Newf(types.ErrCodeInternal, "rabbitmq: close errors: %v", errs)
	}
	return nil
}
