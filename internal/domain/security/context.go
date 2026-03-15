package security

import "context"

// ContextualFactStore retrieves environment-specific facts for policy evaluation.
// Facts include things like the current git branch, environment name, repo criticality, etc.
type ContextualFactStore interface {
	// GetFacts returns contextual facts for the given working directory.
	GetFacts(ctx context.Context, cwd string) (map[string]interface{}, error)
}
