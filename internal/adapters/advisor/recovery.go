package advisor

import (
	"context"
	"fmt"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
)

// RecoveryAdvisor generates denial explanations and suggests alternative actions.
type RecoveryAdvisor struct {
	intentDetector *IntentDetector
}

// NewRecoveryAdvisor creates a new RecoveryAdvisor.
func NewRecoveryAdvisor() *RecoveryAdvisor {
	return &RecoveryAdvisor{
		intentDetector: NewIntentDetector(),
	}
}

// DetectDestructiveIntent proxies to the IntentDetector.
func (r *RecoveryAdvisor) DetectDestructiveIntent(ctx context.Context, ast domain.ExecutionAST) (float64, string, error) {
	return r.intentDetector.DetectDestructiveIntent(ctx, ast)
}

// ExplainDenial generates a user-friendly explanation for why a command was blocked.
func (r *RecoveryAdvisor) ExplainDenial(_ context.Context, task domain.Task, decision security.PolicyDecision) (string, error) {
	var sb strings.Builder

	sb.WriteString("🚨 Warden blocked this action.\n\n")
	sb.WriteString(fmt.Sprintf("Tool: %s\n", task.Tool))

	if decision.PolicyID != "" {
		sb.WriteString(fmt.Sprintf("Policy: %s\n", decision.PolicyID))
	}

	if len(decision.Reasons) > 0 {
		sb.WriteString("Reasons:\n")
		for _, reason := range decision.Reasons {
			sb.WriteString(fmt.Sprintf("  • %s\n", reason))
		}
	}

	return sb.String(), nil
}

// SuggestRecovery proposes alternative commands or next steps.
func (r *RecoveryAdvisor) SuggestRecovery(_ context.Context, task domain.Task, _ security.PolicyDecision) ([]string, error) {
	suggestions := make([]string, 0)

	// Provide context-aware suggestions based on the tool
	command, _ := task.Args["command"].(string)
	args, _ := task.Args["args"].([]string)

	switch {
	case command == "kubectl" && len(args) > 0 && args[0] == "delete":
		suggestions = append(suggestions,
			"Try 'kubectl drain <node>' before deleting pods in production",
			"Use '--dry-run=client' flag to preview the delete operation",
			"Verify your current namespace with 'kubectl config current-context'",
		)
	case command == "terraform" && len(args) > 0 && args[0] == "destroy":
		suggestions = append(suggestions,
			"Run 'terraform plan -destroy' first to review what will be destroyed",
			"Use '-target' flag to limit the blast radius",
			"Ensure you have a state backup with 'terraform state pull > backup.tfstate'",
		)
	case command == "rm" || (command == "docker" && len(args) > 0 && args[0] == "rm"):
		suggestions = append(suggestions,
			"Use '--dry-run' or list files first before deletion",
			"Consider moving to a trash directory instead of permanent deletion",
		)
	case command == "git" && len(args) > 0 && args[0] == "push" && containsFlag(args, "--force"):
		suggestions = append(suggestions,
			"Use '--force-with-lease' instead of '--force' for safer force pushes",
			"Consider creating a backup branch first",
		)
	default:
		suggestions = append(suggestions,
			"Contact your team lead or security admin for an exception token",
			"Review the active Cedar policies with 'duckops policy list'",
		)
	}

	return suggestions, nil
}

func containsFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
