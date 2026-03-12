package security

import (
	"context"
	"fmt"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
)

// TaskWardenAdapter implements the SecurityGatePort.
// It intercepts every command passing through the Task Engine Pipeline
// and enforces both static sanitization rules and dynamic Cedar policies.
type TaskWardenAdapter struct {
	warden     ports.WardenPort
	normalizer ports.CommandNormalizer
	logger     shared_ports.Logger

	// Static configuration
	allowlist map[string]bool
	blocklist []string // substrings to block, e.g. shell operators
}

// NewTaskWardenAdapter creates a new security gate.
func NewTaskWardenAdapter(w ports.WardenPort, n ports.CommandNormalizer, l shared_ports.Logger) ports.SecurityGatePort {
	// A basic allowlist of completely safe or heavily controlled commands
	safeCmds := []string{
		"ls", "dir", "cat", "type", "pwd", "cd", 
		"grep", "findstr", "git", "echo", "mkdir", "rm", "del",
		"tfsec", "trivy", "semgrep", "gitleaks",
		"cmd.exe", "powershell.exe", // allowed but heavily audited/sanitized below
	}
	
	allowMap := make(map[string]bool)
	for _, cmd := range safeCmds {
		allowMap[cmd] = true
	}

	// Shell metacharacters that allow command chaining or subshells.
	// Since we execute directly via os/exec (not via bash -c), most of these
	// are naturally neutralized by Go, but we still block them specifically
	// if someone tries to run `cmd.exe /c` or `/bin/sh -c`
	blocked := []string{
		"&&", ";", "|", "||", ">", ">>", "<", "$(", "`", "!",
	}

	return &TaskWardenAdapter{
		warden:     w,
		normalizer: n,
		logger:     l,
		allowlist:  allowMap,
		blocklist:  blocked,
	}
}

// Evaluate analyzes the task and blocks execution if rules are violated.
func (g *TaskWardenAdapter) Evaluate(ctx context.Context, task domain.OSTask) error {
	baseCmd := strings.ToLower(task.OriginalCmd)

	// 1. Check Allowlist
	// If the user disabled the allowlist (empty map in a future dynamic config), we skip.
	if len(g.allowlist) > 0 {
		if !g.allowlist[baseCmd] {
			return fmt.Errorf("command %q is not in the system allowlist", task.OriginalCmd)
		}
	}

	// 2. Check Blocked Operators (Injection Prevention)
	for _, arg := range task.Args {
		for _, blocked := range g.blocklist {
			if strings.Contains(arg, blocked) {
				return fmt.Errorf("argument contains blocked shell operator %q", blocked)
			}
		}
	}

	// 3. Cedar Policy Engine Check (via Warden)
	if g.warden != nil {
		evalCmd := task.OriginalCmd
		evalArgs := task.Args

		// Use the normalizer to unwrap shell-specific wrappers
		if g.normalizer != nil {
			evalCmd, evalArgs = g.normalizer.Normalize(evalCmd, evalArgs)
		}

		req := security.ExecutionRequest{
			Command:           task.OriginalCmd,
			Args:              task.Args,
			NormalizedCommand: evalCmd,
			NormalizedArgs:    evalArgs,
			Context: map[string]interface{}{
				"cwd": task.Cwd,
			},
		}

		decision, err := g.warden.EvaluateExecution(ctx, req)
		if err != nil {
			return fmt.Errorf("warden policy evaluation failed: %w", err)
		}

		if !decision.Allowed {
			reason := "blocked by zero-trust policy"
			if len(decision.Reasons) > 0 {
				reason = decision.Reasons[0]
			}
			return fmt.Errorf("execution denied: %s (policy_id: %s)", reason, decision.PolicyID)
		}
	}

	return nil
}
