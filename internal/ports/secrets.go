package ports

import "github.com/SecDuckOps/agent/internal/domain/security"

// SecretScannerPort defines the interface for secret detection and substitution.
// Secrets are detected and replaced with placeholders before reaching any LLM.
// Real values are restored only at execution time. Stateless, thread-safe.
type SecretScannerPort interface {
	// Scan scans text and returns all detected secrets.
	Scan(text string) []security.SecretMatch

	// Scrub replaces all detected secrets with placeholders.
	// Returns scrubbed text and a PlaceholderMap for restoration.
	Scrub(sessionID string, text string) (scrubbed string, pm security.PlaceholderMap)

	// Restore replaces placeholders back with real values.
	Restore(text string, pm security.PlaceholderMap) string
}
