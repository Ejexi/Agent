package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
)

// SecurityAdvisorPort provides non-authoritative security advisory capabilities.
// It explains why actions were denied, detects destructive intent, and suggests recovery paths.
// IMPORTANT: This is advisory only — it cannot override Warden decisions.
type SecurityAdvisorPort interface {
	// DetectDestructiveIntent performs fast-path local detection of destructive commands.
	// Returns a risk level (0.0 = safe, 1.0 = critical) and a human-readable explanation.
	DetectDestructiveIntent(ctx context.Context, ast domain.ExecutionAST) (riskScore float64, explanation string, err error)

	// ExplainDenial generates a user-friendly explanation for a Warden policy denial.
	ExplainDenial(ctx context.Context, task domain.Task, decision security.PolicyDecision) (string, error)

	// SuggestRecovery proposes alternative commands or workflows when a command is denied.
	SuggestRecovery(ctx context.Context, task domain.Task, decision security.PolicyDecision) ([]string, error)
}
