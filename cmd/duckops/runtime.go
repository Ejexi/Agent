package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/gui/tui"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
)

// runTUI launches the premium TUI interface.
func runTUI(k *kernel.Kernel, modelName string, appSessionManager ports.AppSessionManager, eventBus ports.EventBusPort) {
	if err := tui.Run(k, modelName, appSessionManager, eventBus); err != nil {
		fmt.Printf("\n\033[31mError launching TUI: %v\033[0m\n", err)
		os.Exit(1)
	}
}

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

		if strings.HasPrefix(input, "shell ") || input == "shell" {
			// Interactive shell mode
			fmt.Println("\n\033[33mEntering interactive shell mode (type 'exit' to return)\033[0m")
			
			// For simplicity in the REPL, we'll just use the ShellExecutionPort directly
			// or via a special TerminalTool call.
			
			task := domain.OSTask{
				OriginalCmd: "bash", // or cmd.exe on Windows
				UsePTY:      true,
				// We don't have interactive input support yet in the tool, 
				// so this is just a demo of streaming output.
			}
			if runtime.GOOS == "windows" {
				task.OriginalCmd = "cmd.exe"
			}
			if input != "shell" {
				parts := strings.Fields(input)
				task.OriginalCmd = parts[1]
				if len(parts) > 2 {
					task.Args = parts[2:]
				}
			}

			sessionID, err := k.Deps.ShellExecution.Start(context.Background(), task)
			if err != nil {
				fmt.Printf("Error starting shell: %v\n", err)
				continue
			}

			ch, _ := k.Deps.ShellExecution.Subscribe(context.Background(), sessionID)
			
			go func() {
				// Handle input (TODO: real terminal raw mode)
				// For now just allow one-off commands or simple streaming
			}()

			for output := range ch {
				fmt.Print(string(output.Data))
			}
			fmt.Println("\n\033[33mShell session ended.\033[0m")
			continue
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

		result, err := k.ExecuteCompat(context.Background(), task)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		if isShellCommand {
			// 1. Check for AI thoughts/rationale bubbled up from the middleware
			if rationale, ok := result.Data["rationale"].(string); ok && rationale != "" {
				fmt.Printf("\n\033[33m[THOUGHTS]\033[0m %s\n", rationale)
			}

			// 2. Print output directly like a real terminal
			stdout, _ := result.Data["stdout"].(string)
			stderr, _ := result.Data["stderr"].(string)
			
			if stdout != "" {
				fmt.Print(stdout)
			}
			if stderr != "" {
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
