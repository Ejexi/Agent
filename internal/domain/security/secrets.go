package security

import "context"

// ────────────────────────────────────
// Secret Substitution Domain Types
// ────────────────────────────────────

// SecretPattern defines a regex pattern for one secret type.
type SecretPattern struct {
	Name    string `json:"name"`    // e.g., "AWS Access Key"
	Pattern string `json:"pattern"` // regex
	Prefix  string `json:"prefix"`  // placeholder prefix, e.g., "AWS_KEY"
}

// SecretMatch represents a detected secret in text.
type SecretMatch struct {
	PatternName string `json:"pattern_name"`
	Value       string `json:"value"`       // original secret value
	Placeholder string `json:"placeholder"` // replacement placeholder
	StartIndex  int    `json:"start_index"`
	EndIndex    int    `json:"end_index"`
}

// PlaceholderMap tracks placeholder → real value mappings for one scrub pass.
// Scoped to a session — never persisted to disk or sent over the network.
type PlaceholderMap struct {
	SessionID string            `json:"session_id"`
	Mappings  map[string]string `json:"-"` // placeholder → real value (never serialized)
}

// SecretProvider defines an interface to securely resolve vault references dynamically.
// This prevents sensitive credentials from leaking into the LLM context.
type SecretProvider interface {
	Resolve(ctx context.Context, ref string) (string, error)
}
