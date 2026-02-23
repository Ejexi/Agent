package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/adapters/rabbitmq"
	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/shared/protocol"
)

// RunInteractiveMode starts the terminal REPL loop for the DevSecOps agent.
func RunInteractiveMode(k *kernel.Kernel, selectedProvider string, mode string) {
	selectedTool := "chat"

	fmt.Println("\n=======================================================")
	fmt.Println("Agent Kernel Interactive Mode")
	fmt.Printf("Active Provider: [%s]\n", selectedProvider)
	fmt.Printf("Active Tool:     [%s]\n", selectedTool)
	fmt.Println("=======================================================")

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("\n[%s:%s] > ", selectedProvider, selectedTool)

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}

		switch {
		case input == "exit", input == "quit":
			fmt.Println("Goodbye!")
			return

		case strings.HasPrefix(input, "/provider "):
			newProvider := strings.TrimSpace(
				strings.TrimPrefix(input, "/provider "),
			)
			if k.Deps.LLM.Get(newProvider) != nil {
				selectedProvider = newProvider
				fmt.Printf("[System] Provider switched to: %s\n", selectedProvider)
			} else {
				fmt.Printf("[Error] Provider '%s' not found\n", newProvider)
			}
			continue

		case strings.HasPrefix(input, "/tool "):
			selectedTool = strings.TrimSpace(
				strings.TrimPrefix(input, "/tool "),
			)
			fmt.Printf("[System] Tool switched to: %s\n", selectedTool)
			continue
		}

		// =====================================================
		// Execute / Dispatch Task
		// =====================================================

		if mode == "cloud" {
			fmt.Printf("[Cloud] Dispatching request to server...\n")
			// In cloud mode, we need a message bus. If not set, initialize it.
			if k.Deps.MessageBus == nil {
				rabbitURL := os.Getenv("RABBITMQ_URL")
				if rabbitURL == "" {
					rabbitURL = "amqp://guest:guest@localhost:5672/"
				}
				bus, err := rabbitmq.NewAdapter(rabbitmq.Config{URL: rabbitURL})
				if err != nil {
					fmt.Printf("[Error] Failed to connect to server bus: %v\n", err)
					continue
				}
				k.SetMessageBus(bus)
			}

			event := protocol.RawInputReceived{
				ID:        fmt.Sprintf("raw-%d", time.Now().UnixNano()),
				Text:      input,
				Source:    "cli",
				Timestamp: time.Now(),
			}

			err := k.Deps.MessageBus.PublishEvent(context.Background(), protocol.QueueRawInput, event)
			if err != nil {
				fmt.Printf("[Cloud Error] Failed to send to server: %v\n", err)
			} else {
				fmt.Printf("[Cloud] Request sent. Waiting for server processing (check logs).\n")
			}
			continue
		}

		// Standalone Mode (Local execution)
		args := map[string]interface{}{
			"ai_provider": selectedProvider,
			"prompt":      input,
		}

		task := domain.Task{
			ID:   fmt.Sprintf("%s-%d", selectedTool, time.Now().UnixNano()),
			Tool: selectedTool,
			Args: args,
		}

		result, err := k.Execute(context.Background(), task)
		if err != nil {
			fmt.Printf("[Execution Error]: %v\n", err)
			continue
		}

		if !result.Success {
			fmt.Printf("[Task Failed]: %s\n", result.Error)
			continue
		}

		if response, ok := result.Data["response"].(string); ok {
			fmt.Printf("\n[AI]: %s\n", response)
		} else {
			fmt.Printf("\n[Result]: %+v\n", result.Data)
		}
	}
}
