package agent

import (
	"path/filepath"
	"runtime"
	"strings"
)

// Intent is the structured output of ParseIntent.
type Intent struct {
	Action     string   // "scan" | "status" | "prefetch" | "help"
	Target     string   // directory path (default: ".")
	Categories []string // empty = all enabled
	Severity   string   // "" | "high" | "critical"
	OutputFmt  string   // "text" | "json"
}

// ParseIntent maps a natural language string to a structured Intent.
// Unknown input always defaults to a full scan — never returns an error to the user.
func ParseIntent(input string) Intent {
	lower := strings.ToLower(strings.TrimSpace(input))
	intent := Intent{Target: ".", OutputFmt: "text"}

	// ── Action ───────────────────────────────────────────────────────────────
	switch {
	case containsAny(lower, "scan", "check", "audit", "analyze", "look for", "find", "inspect"):
		intent.Action = "scan"
	case containsAny(lower, "status", "docker", "images", "cached", "ready", "running"):
		intent.Action = "status"
	case containsAny(lower, "prefetch", "download", "pull", "prepare", "warmup", "warm up"):
		intent.Action = "prefetch"
	case containsAny(lower, "help", "what can", "how do", "commands", "usage"):
		intent.Action = "help"
	default:
		intent.Action = "scan" // always default to scan
	}

	// ── Target path ───────────────────────────────────────────────────────────
	if path := extractPath(input); path != "" {
		intent.Target = normalizePath(path)
	}

	// ── Category detection ────────────────────────────────────────────────────
	categoryMap := map[string][]string{
		"sast":    {"sast", "static", "code analysis", "semgrep", "codeql", "gosec", "bandit"},
		"sca":     {"sca", "dependencies", "packages", "cve", "trivy", "grype", "supply chain"},
		"secrets": {"secret", "secrets", "keys", "tokens", "passwords", "hardcoded", "gitleaks", "credentials"},
		"iac":     {"iac", "infrastructure", "terraform", "kubernetes", "k8s", "checkov", "tfsec"},
		"deps":    {"deps", "outdated", "pip", "npm", "dependency"},
	}
	for cat, keywords := range categoryMap {
		if containsAny(lower, keywords...) {
			intent.Categories = append(intent.Categories, cat)
		}
	}

	// ── Severity filter ───────────────────────────────────────────────────────
	switch {
	case containsAny(lower, "critical only", "only critical", "just critical", "criticals"):
		intent.Severity = "critical"
	case containsAny(lower, "high only", "only high", "just high", "high and above"):
		intent.Severity = "high"
	case containsAny(lower, "medium", "med"):
		intent.Severity = "medium"
	}

	// ── Output format ─────────────────────────────────────────────────────────
	if containsAny(lower, "json", "machine readable", "export", "output json") {
		intent.OutputFmt = "json"
	}

	return intent
}

// normalizePath handles Windows LLM hallucinations (e.g. /app → .)
// and cleans the path.
func normalizePath(path string) string {
	if runtime.GOOS == "windows" {
		switch path {
		case "/app", "/workspace", "/code", "/project", "/src", "/vuln":
			return "."
		}
	}
	return filepath.Clean(path)
}

// resolvePath resolves a possibly-relative path against cwd.
func resolvePath(input string, cwd string) string {
	input = normalizePath(input)
	if filepath.IsAbs(input) {
		return input
	}
	return filepath.Join(cwd, input)
}

// extractPath tries to find a file/directory path in the input string.
// Looks for patterns like "the src folder", "./backend", "/path/to/dir".
func extractPath(input string) string {
	words := strings.Fields(input)
	for i, word := range words {
		// Explicit relative or absolute paths
		if strings.HasPrefix(word, "./") || strings.HasPrefix(word, "/") || strings.HasPrefix(word, "../") {
			return word
		}
		// "the X folder/directory" pattern
		if (word == "the" || word == "in" || word == "at") && i+1 < len(words) {
			next := words[i+1]
			if i+2 < len(words) && (words[i+2] == "folder" || words[i+2] == "directory" || words[i+2] == "dir") {
				return "./" + next
			}
		}
	}
	return ""
}

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
