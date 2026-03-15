package engine

import (
	"os/exec"
	"strings"
)

// ConfidenceLevel represents how confident the router is about intent classification.
type ConfidenceLevel float64

const (
	ConfidenceHigh   ConfidenceLevel = 0.9
	ConfidenceMedium ConfidenceLevel = 0.5
	ConfidenceLow    ConfidenceLevel = 0.2
)

// RouteDecision describes the result of intent classification.
type RouteDecision struct {
	Intent     string          // "terminal", "chat", "orchestration"
	Confidence ConfidenceLevel
	Command    string   // The detected command (if terminal)
	Args       []string // Parsed arguments (if terminal)
}

// Router classifies user input into execution intents using heuristic-first approach.
type Router struct {
	cwd string
}

// NewRouter creates a new intent router.
func NewRouter(cwd string) *Router {
	return &Router{cwd: cwd}
}

// Classify determines the execution intent of user input.
func (r *Router) Classify(input string) RouteDecision {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return RouteDecision{Intent: "chat", Confidence: ConfidenceHigh}
	}

	command := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	// 1. Check if the command exists in OS PATH (highest confidence)
	if _, err := exec.LookPath(command); err == nil {
		return RouteDecision{
			Intent:     "terminal",
			Confidence: ConfidenceHigh,
			Command:    command,
			Args:       args,
		}
	}

	// 2. Check shell builtins
	builtins := map[string]bool{
		"cd": true, "pwd": true, "echo": true,
		"type": true, "dir": true, "cls": true, "clear": true,
	}
	if builtins[command] {
		return RouteDecision{
			Intent:     "terminal",
			Confidence: ConfidenceHigh,
			Command:    command,
			Args:       args,
		}
	}

	// 3. Check for orchestration keywords (multi-step workflow patterns)
	orchestrationKeywords := []string{"deploy", "rollout", "migrate", "pipeline", "workflow"}
	lowerInput := strings.ToLower(input)
	for _, kw := range orchestrationKeywords {
		if strings.Contains(lowerInput, kw) {
			return RouteDecision{
				Intent:     "orchestration",
				Confidence: ConfidenceMedium,
			}
		}
	}

	// 4. Fallback to chat (LLM reasoning)
	return RouteDecision{
		Intent:     "chat",
		Confidence: ConfidenceLow,
	}
}
