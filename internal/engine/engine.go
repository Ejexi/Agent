package engine

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/kernel"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
	"path/filepath"
)

// Engine is the bridge between the TUI and the Kernel.
type Engine struct {
	kernel *kernel.Kernel
	cwd    string
}

// NewEngine creates a new TUI bridge engine.
// In a real scenario, this would be injected by the bootstrap.
func NewEngine(cwd string) *Engine {
	return &Engine{
		cwd: cwd,
	}
}

// GetCwd returns current working directory in the engin
func (e *Engine) GetCwd() string {
	return e.cwd
}

// SetKernel allows injecting the initialized kernel.
func (e *Engine) SetKernel(k *kernel.Kernel) {
	e.kernel = k
}

// ChatResult contains the content and metadata for a chat interaction.
type ChatResult struct {
	Content string
	Model   string
	Usage   shared_domain.TokenUsage
}

// Chat processes a user prompt through the DuckOps kernel.
func (e *Engine) Chat(ctx context.Context, input string) (ChatResult, error) {
	if e.kernel == nil {
		return ChatResult{}, fmt.Errorf("kernel not initialized")
	}

	// Detect if this is a shell command dynamically
	isShellCommand := false
	
	cmdParts := strings.Fields(input)
	if len(cmdParts) > 0 {
		command := cmdParts[0]
		
		// Check if the command exists in the system PATH
		// If not, check against essential shell built-ins
		if _, err := exec.LookPath(command); err == nil {
			isShellCommand = true
		} else {
			builtinCommands := map[string]bool{
				"cd": true, "pwd": true, "echo": true, "type": true, "dir": true, "cls": true,
				"clear": true, "ls": true, "cat": true, "mkdir": true, "rm": true, "git": true,
				"go": true, "docker": true, "kubectl": true,
			}
			if builtinCommands[command] {
				isShellCommand = true
			}
		}
	}

	var task domain.Task
	if isShellCommand {
		command := cmdParts[0]
		var args []string
		if len(cmdParts) > 1 {
			args = cmdParts[1:]
		}

		task = domain.Task{
			ID:   "tui_task",
			Tool: "terminal",
			Args: map[string]interface{}{
				"command": command,
				"args":    args,
				"cwd":     e.cwd,
			},
			RequiredCaps: []security.Capability{security.CapExecuteShell},
		}
	} else if strings.HasPrefix(input, "/") {
		cmdParts := strings.Fields(strings.TrimPrefix(input, "/"))
		command := cmdParts[0]

		if command == "scan" {
			target := e.cwd
			if len(cmdParts) > 1 {
				target = cmdParts[1]
			}
			task = domain.Task{
				ID:   "tui_scan",
				Tool: "scan",
				Args: map[string]interface{}{
					"target": target,
				},
			}
		} else {
			// fallback for other slash commands: send to chat but as system prompt
			task = domain.Task{
				ID:   "tui_chat",
				Tool: "chat",
				Args: map[string]interface{}{
					"prompt": fmt.Sprintf("System Command Issued: %s. Respond as the DuckOps DevSecOps Assistant with the appropriate internal diagnostic or action representation.", input),
				},
			}
		}
	} else {
		task = domain.Task{
			ID:   "tui_chat",
			Tool: "chat",
			Args: map[string]interface{}{
				"prompt": input,
			},
		}
	}

	// Wrap context with default user capabilities
	execCtx := kernel.NewExecutionContext(ctx, "user:tui", []security.Capability{
		security.CapReadFS,
		security.CapExecuteShell,
	})

	result, err := e.kernel.Execute(execCtx, task)
	if err != nil {
		return ChatResult{}, err
	}

	res := ChatResult{}
	
	// Extract basic metadata
	if usage, ok := result.Data["usage"].(shared_domain.TokenUsage); ok {
		res.Usage = usage
	}
	if model, ok := result.Data["model"].(string); ok {
		res.Model = model
	}

	if isShellCommand {
		// Priority 1: AI Result presentation (Beautified)
		if reflection, ok := result.Data["stdout"].(string); ok && reflection != "" {
			res.Content = reflection
			return res, nil
		}
		
		// Fallback to raw output if no beautification
		stdout, _ := result.Data["raw_stdout"].(string)
		if stdout == "" {
			stdout, _ = result.Data["stdout"].(string)
		}
		stderr, _ := result.Data["stderr"].(string)
		
		output := stdout
		if stderr != "" {
			output += "\n" + stderr
		}
		res.Content = output
		return res, nil
	}

	// For chat tool
	if resp, ok := result.Data["response"].(string); ok {
		res.Content = resp
		return res, nil
	}

	res.Content = fmt.Sprintf("Success: %v", result.Data)
	return res, nil
}

// GetSuggestions returns a dynamic set of suggested questions for the user.
func (e *Engine) GetSuggestions(ctx context.Context) []string {
	pool := []string{
		"Analyze current project architecture",
		"Scan for vulnerabilities in " + filepath.Base(e.cwd),
		"Show me recent git changes",
		"How can I improve my security setup?",
		"Review the latest file changes",
		"Check for hardcoded secrets",
		"Explain DuckOps core features",
		"Optimize my terminal workflow",
		"Show project dependencies",
		"List all security capabilities",
	}

	// Simple shuffle
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})

	// Return top 3-4 suggestions
	limit := 3
	if len(pool) < limit {
		limit = len(pool)
	}
	return pool[:limit]
}

