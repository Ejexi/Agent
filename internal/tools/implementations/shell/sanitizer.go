package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/SecDuckOps/shared/types"
)

// dangerousPatterns detects shell metacharacters and injection attempts.
var dangerousPatterns = regexp.MustCompile(`[;&|` + "`" + `$(){}><!\n\r]`)

// SanitizeResult holds the sanitized path and any workspace info.
type SanitizeResult struct {
	CleanPath     string
	WorkspaceRoot string
}

// SanitizePath validates and resolves a path, ensuring it stays within the workspace boundary.
// Returns an error if the path is empty, contains traversal patterns, or escapes the workspace.
func SanitizePath(rawPath, workspaceRoot string) (SanitizeResult, error) {
	if strings.TrimSpace(rawPath) == "" {
		return SanitizeResult{}, types.New(types.ErrCodeInvalidInput, "path is required")
	}

	// Resolve workspace root to absolute
	absWorkspace, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return SanitizeResult{}, types.Wrap(err, types.ErrCodeInternal, "failed to resolve workspace root")
	}

	// Resolve the target path
	var absPath string
	if filepath.IsAbs(rawPath) {
		absPath = filepath.Clean(rawPath)
	} else {
		absPath = filepath.Clean(filepath.Join(absWorkspace, rawPath))
	}

	// Evaluate symlinks to prevent symlink escape
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If the path doesn't exist yet (e.g., write/mkdir), evaluate the parent
		parentDir := filepath.Dir(absPath)
		evalParent, evalErr := filepath.EvalSymlinks(parentDir)
		if evalErr != nil {
			// Parent doesn't exist either — check against the raw cleaned path
			evalPath = absPath
		} else {
			evalPath = filepath.Join(evalParent, filepath.Base(absPath))
		}
	}

	// Ensure the resolved path is within the workspace boundary
	evalWorkspace, err := filepath.EvalSymlinks(absWorkspace)
	if err != nil {
		evalWorkspace = absWorkspace
	}

	if !isSubPath(evalPath, evalWorkspace) {
		return SanitizeResult{}, types.Newf(types.ErrCodePermissionDenied,
			"path %q escapes workspace boundary %q", rawPath, workspaceRoot)
	}

	return SanitizeResult{
		CleanPath:     evalPath,
		WorkspaceRoot: evalWorkspace,
	}, nil
}

// SanitizeArgs checks command arguments for dangerous patterns.
// Returns an error if any argument contains shell metacharacters.
func SanitizeArgs(args []string) error {
	for i, arg := range args {
		if dangerousPatterns.MatchString(arg) {
			return types.Newf(types.ErrCodePermissionDenied,
				"argument %d contains forbidden characters: %q", i, arg)
		}
	}
	return nil
}

// SanitizeCommand validates a command name against the allowed commands list.
// Returns the resolved command (potentially cross-platform mapped).
func SanitizeCommand(cmd string, allowed map[string]CommandMapping) (CommandMapping, error) {
	cmd = strings.TrimSpace(strings.ToLower(cmd))
	if cmd == "" {
		return CommandMapping{}, types.New(types.ErrCodeInvalidInput, "command is required")
	}

	mapping, ok := allowed[cmd]
	if !ok {
		return CommandMapping{}, types.Newf(types.ErrCodePermissionDenied,
			"command %q is not in the allowed commands list", cmd)
	}

	return mapping, nil
}

// isSubPath checks if child is a subdirectory of parent.
func isSubPath(child, parent string) bool {
	// Normalize separators for comparison
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)

	// Handle case-insensitive file systems (Windows)
	if isWindows() {
		child = strings.ToLower(child)
		parent = strings.ToLower(parent)
	}

	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", the child is outside the parent
	return !strings.HasPrefix(rel, "..") && rel != "."  || rel == "."
}

// isWindows returns true if the current OS is Windows.
func isWindows() bool {
	return os.PathSeparator == '\\'
}

// ValidateWorkspace checks that a workspace root exists and is a directory.
func ValidateWorkspace(workspaceRoot string) error {
	info, err := os.Stat(workspaceRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return types.Newf(types.ErrCodeInvalidInput, "workspace %q does not exist", workspaceRoot)
		}
		return types.Wrap(err, types.ErrCodeInternal, fmt.Sprintf("failed to stat workspace %q", workspaceRoot))
	}
	if !info.IsDir() {
		return types.Newf(types.ErrCodeInvalidInput, "workspace %q is not a directory", workspaceRoot)
	}
	return nil
}
