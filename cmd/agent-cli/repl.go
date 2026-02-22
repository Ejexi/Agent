package main

import (
	"duckops/internal/domain"
	"duckops/internal/kernel"
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// RunInteractiveMode starts the terminal REPL loop for the DevSecOps agent.
func RunInteractiveMode(k *kernel.Kernel, selectedProvider string) {
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
		// Execute Task
		// =====================================================
		args := map[string]interface{}{
			"ai_provider": selectedProvider,
			"prompt":      input,
		}

		task := domain.Task{
			ID: fmt.Sprintf("%s-%d", selectedTool, time.Now().UnixNano()),
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
