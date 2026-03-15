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

// StreamChat processes a user prompt and returns a channel of events (Thinking, ToolCalls, FinalResult).
func (e *Engine) StreamChat(ctx context.Context, input string) (<-chan any, error) {
	if e.kernel == nil {
		return nil, fmt.Errorf("kernel not initialized")
	}

	eventCh := make(chan any, 10)
	
	go func() {
		defer close(eventCh)

		// 1. Prepare Task (same logic as Chat, but simplified for brevity)
		task := e.prepareTask(input)
		
		// 2. Wrap context with event callback
		execCtx := kernel.NewExecutionContext(ctx, "session:tui", "user:tui", []security.Capability{
			security.CapReadFS,
			security.CapExecuteShell,
		}).WithEventCallback(func(evt any) {
			eventCh <- evt
		})

		// 3. Execute
		result, err := e.kernel.Execute(execCtx, task)
		if err != nil {
			eventCh <- err
			return
		}

		// 4. Return Final Result
		res := ChatResult{}
		if usage, ok := result.Data["usage"].(shared_domain.TokenUsage); ok {
			res.Usage = usage
		}
		if model, ok := result.Data["model"].(string); ok {
			res.Model = model
		}
		if resp, ok := result.Data["response"].(string); ok {
			res.Content = resp
		} else {
			res.Content = fmt.Sprintf("Success: %v", result.Data)
		}
		
		eventCh <- res
	}()

	return eventCh, nil
}

// prepareTask is a private helper to deduplicate task preparation logic.
func (e *Engine) prepareTask(input string) domain.Task {
	isShellCommand := false
	cmdParts := strings.Fields(input)
	if len(cmdParts) > 0 {
		command := cmdParts[0]
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

	if isShellCommand {
		return domain.Task{
			ID:   "tui_task",
			Tool: "terminal",
			Args: map[string]interface{}{
				"command": cmdParts[0],
				"args":    cmdParts[1:],
				"cwd":     e.cwd,
			},
			RequiredCaps: []security.Capability{security.CapExecuteShell},
		}
	}
	
	if strings.HasPrefix(input, "/") {
		// Handle / commands (simplified)
		return domain.Task{
			ID:   "tui_chat",
			Tool: "chat",
			Args: map[string]interface{}{
				"prompt": input,
			},
		}
	}

	return domain.Task{
		ID:   "tui_chat",
		Tool: "chat",
		Args: map[string]interface{}{
			"prompt": input,
		},
	}
}

// Chat processes a user prompt through the DuckOps kernel (Synchronous wrapper).
func (e *Engine) Chat(ctx context.Context, input string) (ChatResult, error) {
	ch, err := e.StreamChat(ctx, input)
	if err != nil {
		return ChatResult{}, err
	}

	var lastRes ChatResult
	for evt := range ch {
		if res, ok := evt.(ChatResult); ok {
			lastRes = res
		} else if err, ok := evt.(error); ok {
			return ChatResult{}, err
		}
	}

	return lastRes, nil
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

