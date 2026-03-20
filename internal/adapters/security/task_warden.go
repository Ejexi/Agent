package security

import (
	"context"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/types"
)

// TaskWardenAdapter implements the SecurityGatePort.
// It intercepts every command passing through the Task Engine Pipeline
// and enforces both static sanitization rules and dynamic Cedar policies.
type TaskWardenAdapter struct {
	warden     ports.WardenPort
	normalizer ports.CommandNormalizer
	logger     shared_ports.Logger

	blocklist []string // substrings to block, e.g. shell operators
}

// NewTaskWardenAdapter creates a new security gate.
func NewTaskWardenAdapter(w ports.WardenPort, n ports.CommandNormalizer, l shared_ports.Logger) ports.SecurityGatePort {
	// Shell metacharacters that allow command chaining or subshells.
	blocked := []string{
		"&&", ";", "|", "||", ">", ">>", "<", "$(", "`", "!","{}",
	}

	return &TaskWardenAdapter{
		warden:     w,
		normalizer: n,
		logger:     l,
		blocklist:  blocked,
	}
}

// Evaluate analyzes the task and blocks execution if rules are violated.
func (g *TaskWardenAdapter) Evaluate(ctx context.Context, task domain.OSTask) error {
	// 2. Check Blocked Operators (Injection Prevention)
	for _, arg := range task.Args {
		for _, blocked := range g.blocklist {
			if strings.Contains(arg, blocked) {
				return types.Newf(types.ErrCodeInvalidInput, "argument contains blocked shell operator %q", blocked)
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
			return types.Wrapf(err, types.ErrCodeInternal, "warden policy evaluation failed")
		}

		if !decision.Allowed {
			reason := "blocked by zero-trust policy"
			if len(decision.Reasons) > 0 {
				reason = decision.Reasons[0]
			}

			if g.logger != nil {
				g.logger.Info(ctx, "Execution denied by Warden",
					shared_ports.Field{Key: "command", Value: task.OriginalCmd},
					shared_ports.Field{Key: "policy_id", Value: decision.PolicyID},
					shared_ports.Field{Key: "reason", Value: reason},
				)
			}

			return types.Newf(types.ErrCodePermissionDenied, "execution denied: %s (policy_id: %s)", reason, decision.PolicyID)
		}
	}

	return nil
}
