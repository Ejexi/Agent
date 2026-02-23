package rabbitmq

import (
	"context"
	"duckops/internal/domain"
	"encoding/json"
	"fmt"
	"log"
	"sync"

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
		return nil, fmt.Errorf("rabbitmq: failed to connect: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: failed to open channel: %w", err)
	}

	return &Adapter{
		conn:    conn,
		channel: ch,
	}, nil
}

// Publish serializes a domain.Result to JSON and sends it to the specified topic.
// Serialization is the adapter's responsibility — the kernel never touches wire formats.
func (a *Adapter) Publish(ctx context.Context, topic string, result domain.Result) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Adapter handles serialization
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("rabbitmq: failed to marshal result: %w", err)
	}

	// Ensure the queue exists (idempotent)
	_, err = a.channel.QueueDeclare(
		topic, // queue name
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("rabbitmq: failed to declare queue %q: %w", topic, err)
	}

	return a.channel.PublishWithContext(ctx,
		"",    // exchange (default direct exchange)
		topic, // routing key = queue name
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         data,
		},
	)
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
		return fmt.Errorf("rabbitmq: failed to declare queue %q: %w", topic, err)
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
		return fmt.Errorf("rabbitmq: failed to start consumer on %q: %w", topic, err)
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
			errs = append(errs, fmt.Errorf("rabbitmq: channel close: %w", err))
		}
	}
	if a.conn != nil {
		if err := a.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("rabbitmq: connection close: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("rabbitmq: close errors: %v", errs)
	}
	return nil
}
