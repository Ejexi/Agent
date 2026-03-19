// Package search provides code and file search tools.

package search

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/types"
)

// GrepSearchParams are the parameters for the grep_search tool.
type GrepSearchParams struct {
	// Pattern is the search string or regex.
	Pattern string `json:"pattern"`
	// Path is the directory or file to search. Defaults to "." (CWD).
	Path string `json:"path,omitempty"`
	// Glob restricts the search to files matching a glob pattern.
	// Example: "*.go", "**/*.yaml"
	Glob string `json:"glob,omitempty"`
	// CaseInsensitive enables case-insensitive matching.
	CaseInsensitive bool `json:"case_insensitive,omitempty"`
	// MaxResults caps the number of matching lines returned. Default 100.
	MaxResults int `json:"max_results,omitempty"`
}

// GrepSearchTool searches code and files using ripgrep (rg) with a grep fallback.
// Mirrors duckops's grep_search which uses the grep_regex + grep_searcher crates.
type GrepSearchTool struct {
	base.BaseTypedTool[GrepSearchParams]
}

func NewGrepSearchTool() *GrepSearchTool {
	t := &GrepSearchTool{}
	t.Impl = t
	return t
}

func (t *GrepSearchTool) ParseParams(input map[string]interface{}) (GrepSearchParams, error) {
	return base.DefaultParseParams[GrepSearchParams](input)
}

func (t *GrepSearchTool) Name() string { return "grep_search" }

func (t *GrepSearchTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "grep_search",
		Description: `Search for a pattern in files using regex. Returns file paths, line numbers, and matching lines.
Use this to:
- Find where a function, variable, or string is defined or used
- Locate all files containing a specific import or dependency
- Search for TODO/FIXME comments
- Find configuration values across the codebase

Prefer this over running raw grep in the terminal — it respects .gitignore and formats output clearly.`,
		Parameters: map[string]string{
			"pattern":          "string (required) — regex or literal string to search for",
			"path":             "string (optional) — directory or file to search, defaults to current directory",
			"glob":             "string (optional) — file glob filter, e.g. '*.go', '**/*.yaml'",
			"case_insensitive": "bool (optional) — case-insensitive search, default false",
			"max_results":      "int (optional) — max matching lines to return, default 100",
		},
	}
}

func (t *GrepSearchTool) Execute(_ context.Context, params GrepSearchParams) (domain.Result, error) {
	if params.Pattern == "" {
		return domain.Result{}, types.New(types.ErrCodeInvalidInput, "pattern is required")
	}

	searchPath := "."
	if params.Path != "" {
		searchPath = params.Path
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		absPath = searchPath
	}

	// Verify path exists
	if _, err := os.Stat(absPath); err != nil {
		return domain.Result{
			Success: false,
			Status:  "path not found",
			Data:    map[string]interface{}{"error": fmt.Sprintf("path %q does not exist", absPath)},
		}, nil
	}

	maxResults := 100
	if params.MaxResults > 0 {
		maxResults = params.MaxResults
	}

	output, tool := runSearch(params.Pattern, absPath, params.Glob, params.CaseInsensitive, maxResults)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var results []string
	for _, l := range lines {
		if l != "" {
			results = append(results, l)
		}
	}

	return domain.Result{
		Success: true,
		Status:  fmt.Sprintf("found %d matches", len(results)),
		Data: map[string]interface{}{
			"matches":     results,
			"match_count": len(results),
			"pattern":     params.Pattern,
			"path":        absPath,
			"tool_used":   tool,
		},
	}, nil
}

// runSearch tries ripgrep first (faster, respects .gitignore), falls back to grep.
func runSearch(pattern, path, glob string, caseInsensitive bool, maxResults int) (string, string) {
	// Try ripgrep
	if rg, err := exec.LookPath("rg"); err == nil {
		args := []string{"--line-number", "--no-heading", "--color=never",
			fmt.Sprintf("--max-count=%d", maxResults)}
		if caseInsensitive {
			args = append(args, "-i")
		}
		if glob != "" {
			args = append(args, "--glob", glob)
		}
		args = append(args, pattern, path)

		var out bytes.Buffer
		cmd := exec.Command(rg, args...)
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil || out.Len() > 0 {
			return out.String(), "ripgrep"
		}
	}

	// Fall back to grep
	args := []string{"-rn", "--include=*", "-m", fmt.Sprintf("%d", maxResults)}
	if caseInsensitive {
		args = append(args, "-i")
	}
	if glob != "" {
		// Convert glob to --include flag
		args = append(args, fmt.Sprintf("--include=%s", filepath.Base(glob)))
	}
	args = append(args, pattern, path)

	var out bytes.Buffer
	cmd := exec.Command("grep", args...)
	cmd.Stdout = &out
	_ = cmd.Run()
	return out.String(), "grep"
}
