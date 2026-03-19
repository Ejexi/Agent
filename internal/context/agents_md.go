package context

import (
	"fmt"
	"os"
	"path/filepath"
)

// AgentsMD represents a discovered AGENTS.md file.
// Mirrors duckops cli/src/utils/agents_md.rs
type AgentsMD struct {
	Content string
	Path    string
}

// DiscoverAgentsMD walks up from startDir (max 5 levels) looking for AGENTS.md.
// The nearest file wins (closest to startDir).
// Checks: AGENTS.md, agents.md (case-insensitive fallback).
func DiscoverAgentsMD(startDir string) *AgentsMD {
	dir := startDir
	for i := 0; i <= 5; i++ {
		for _, name := range []string{"AGENTS.md", "agents.md"} {
			path := filepath.Join(dir, name)
			if b, err := os.ReadFile(path); err == nil {
				return &AgentsMD{Content: string(b), Path: path}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
}

// FormatForPrompt formats the AGENTS.md content for injection into system prompt.
func (a *AgentsMD) FormatForPrompt() string {
	return fmt.Sprintf("# AGENTS.md (from %s)\n\n%s", a.Path, a.Content)
}
