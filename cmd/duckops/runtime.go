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

		// Detect if this is a raw shell command to bypass the LLM
		isShellCommand := false
		shellCommands := []string{"ls", "cd", "pwd", "cat", "echo", "mkdir", "rm", "git", "go", "docker", "kubectl", "dir", "type"}
		
		cmdParts := strings.Fields(input)
		if len(cmdParts) > 0 {
			for _, sc := range shellCommands {
				if cmdParts[0] == sc {
					isShellCommand = true
					break
				}
			}
		}

		var task domain.Task
		if isShellCommand {
			// Extract command and args
			command := cmdParts[0]
			var args []string
			if len(cmdParts) > 1 {
				args = cmdParts[1:]
			}

			task = domain.Task{
				ID:   fmt.Sprintf("repl_%d", os.Getpid()),
				Tool: "terminal",
				Args: map[string]interface{}{
					"command": command,
					"args":    args,
					"cwd":     "", // the pipeline handles this
				},
			}
		} else {
			task = domain.Task{
				ID:   fmt.Sprintf("repl_%d", os.Getpid()),
				Tool: "chat",
				Args: map[string]interface{}{
					"prompt":      input,
					"ai_provider": provider,
				},
			}
		}

		result, err := k.Execute(context.Background(), task)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		if isShellCommand {
			// Print directly like a real terminal
			if stdout, ok := result.Data["stdout"].(string); ok && stdout != "" {
				fmt.Print(stdout)
			}
			if stderr, ok := result.Data["stderr"].(string); ok && stderr != "" {
				fmt.Print(stderr)
			}
			if !result.Success {
				fmt.Printf("\n\033[31mCommand failed: %s\033[0m\n", result.Error)
			}
			continue
		}

		// Extract the response text for Chat tool
		if resp, ok := result.Data["response"].(string); ok {
			fmt.Printf("\n\033[32m[%s]\033[0m\n\033[32m%s\033[0m\n", strings.ToUpper(string(shared_domain.RoleAssistant)), resp)
			continue
		}
		resultJSON, _ := json.MarshalIndent(result.Data, "", "  ")
		fmt.Printf("\n\033[32m[%s]\033[0m\n\033[32m%s\033[0m\n", strings.ToUpper(string(shared_domain.RoleAssistant)), string(resultJSON))
	}
}
