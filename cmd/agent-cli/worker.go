package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"duckops/internal/adapters/rabbitmq"
	"duckops/internal/kernel"
)

// RunWorkerMode starts the agent as a background worker processing tasks.
func RunWorkerMode(k *kernel.Kernel, workerType string) {
	fmt.Printf("Starting worker mode: [%s]\n", workerType)

	// 1. Get Message Bus Connection String
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	// 2. Initialize RabbitMQ Adapter
	log.Printf("Connecting to RabbitMQ at %s...", rabbitURL)
	busAdapter, err := rabbitmq.NewAdapter(rabbitmq.Config{
		URL: rabbitURL,
	})
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer busAdapter.Close()

	// 3. Start Dispatcher via Kernel
	k.SetMessageBus(busAdapter)

	// Topic format e.g. "tasks.sast"
	topic := fmt.Sprintf("tasks.%s", workerType)
	log.Printf("Subscribing to topic: %s", topic)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := k.StartDispatcher(ctx, topic); err != nil {
		log.Fatalf("Dispatcher failed to start: %v", err)
	}

	log.Printf("Worker [%s] is ready and listening for tasks.", workerType)

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Received signal: %v. Shutting down worker [%s]...", sig, workerType)
}
