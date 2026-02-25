package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/kernel"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
)

// runInteractive starts the REPL loop.
func runInteractive(k *kernel.Kernel, provider, mode string) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("\n🦆 DuckOps Agent (provider: %s, mode: %s)\n", provider, mode)
	fmt.Println("Type a command or 'exit' to quit.")

	for {
		fmt.Printf("\n\033[36m[%s] duckops> \033[0m", strings.ToUpper(string(shared_domain.RoleUser)))
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		task := domain.Task{
			ID:   fmt.Sprintf("repl_%d", os.Getpid()),
			Tool: "chat",
			Args: map[string]interface{}{
				"prompt":      input,
				"ai_provider": provider,
			},
		}

		result, err := k.Execute(context.Background(), task)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Extract the response text
		if resp, ok := result.Data["response"].(string); ok {
			fmt.Printf("\n\033[32m[%s]\033[0m\n\033[32m%s\033[0m\n", strings.ToUpper(string(shared_domain.RoleAssistant)), resp)
			continue
		}
		resultJSON, _ := json.MarshalIndent(result.Data, "", "  ")
		fmt.Printf("\n\033[32m[%s]\033[0m\n\033[32m%s\033[0m\n", strings.ToUpper(string(shared_domain.RoleAssistant)), string(resultJSON))
	}
}
