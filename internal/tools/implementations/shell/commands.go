package shell

import "runtime"

// CommandMapping defines a cross-platform command mapping.
type CommandMapping struct {
	// Name is the canonical command name used by the agent.
	Name string

	// Description explains what this command does.
	Description string

	// Linux is the command binary on Linux.
	Linux string

	// Windows is the command binary on Windows.
	Windows string

	// Darwin is the command binary on macOS.
	Darwin string

	// RequiresArgs indicates the command needs at least one argument.
	RequiresArgs bool

	// AllowedFlags are the flags that are safe to pass to this command.
	// Empty means all flags are allowed (for simple commands).
	AllowedFlags []string
}

// Binary returns the correct binary name for the current OS.
func (c CommandMapping) Binary() string {
	switch runtime.GOOS {
	case "windows":
		if c.Windows != "" {
			return c.Windows
		}
	case "darwin":
		if c.Darwin != "" {
			return c.Darwin
		}
		// Fall through to Linux (macOS shares most commands)
		return c.Linux
	}
	return c.Linux
}

// AllowedCommands is the registry of all permitted commands.
// Only commands in this map can be executed by ShellTool.
// The keys are the canonical names used by the agent.
var AllowedCommands = map[string]CommandMapping{
	// ── File listing ──
	"ls": {
		Name:        "ls",
		Description: "List directory contents",
		Linux:       "ls",
		Windows:     "dir",
		Darwin:      "ls",
	},

	// ── File reading ──
	"cat": {
		Name:         "cat",
		Description:  "Display file contents",
		Linux:        "cat",
		Windows:      "cat",
		Darwin:       "cat",
		RequiresArgs: true,
	},

	// ── File search ──
	"find": {
		Name:        "find",
		Description: "Search for files and directories",
		Linux:       "find",
		Windows:     "cmd",
		Darwin:      "find",
	},

	// ── Text search ──
	"grep": {
		Name:         "grep",
		Description:  "Search file contents for patterns",
		Linux:        "grep",
		Windows:      "findstr",
		Darwin:       "grep",
		RequiresArgs: true,
	},

	// ── Word count / stats ──
	"wc": {
		Name:        "wc",
		Description: "Count lines, words, and characters",
		Linux:       "wc",
		Windows:     "cmd",
		Darwin:      "wc",
	},

	// ── File info ──
	"stat": {
		Name:         "stat",
		Description:  "Display file status/metadata",
		Linux:        "stat",
		Windows:      "cmd",
		Darwin:       "stat",
		RequiresArgs: true,
	},

	// ── Head / Tail ──
	"head": {
		Name:         "head",
		Description:  "Output the first part of files",
		Linux:        "head",
		Windows:      "cmd",
		Darwin:       "head",
		RequiresArgs: true,
	},
	"tail": {
		Name:         "tail",
		Description:  "Output the last part of files",
		Linux:        "tail",
		Windows:      "cmd",
		Darwin:       "tail",
		RequiresArgs: true,
	},

	// ── Git ──
	"git": {
		Name:        "git",
		Description: "Git version control operations",
		Linux:       "git",
		Windows:     "git",
		Darwin:      "git",
		AllowedFlags: []string{
			"status", "log", "diff", "show", "branch", "tag",
			"ls-files", "rev-parse", "describe", "remote",
			"--oneline", "--short", "--name-only", "--stat",
			"-n", "-1", "-v",
		},
	},

	// ── Directory tree ──
	"tree": {
		Name:        "tree",
		Description: "Display directory tree structure",
		Linux:       "tree",
		Windows:     "tree",
		Darwin:      "tree",
	},

	// ── Disk usage ──
	"du": {
		Name:        "du",
		Description: "Estimate file space usage",
		Linux:       "du",
		Windows:     "cmd",
		Darwin:      "du",
	},

	// ── Which / Where ──
	"which": {
		Name:        "which",
		Description: "Locate a command binary",
		Linux:       "which",
		Windows:     "where",
		Darwin:      "which",
	},
}

// WindowsShellArgs converts a canonical command + args into Windows cmd.exe
// compatible arguments when the command maps to "cmd".
func WindowsShellArgs(canonical string, args []string) []string {
	switch canonical {
	case "ls":
		return append([]string{"/C", "dir"}, args...)
	case "cat":
		return append([]string{"/C", "type"}, args...)
	case "find":
		return append([]string{"/C", "dir", "/s", "/b"}, args...)
	case "wc":
		// Approximate wc with find /c
		return append([]string{"/C", "find", "/c", "/v", `""`}, args...)
	case "stat":
		return append([]string{"/C", "dir"}, args...)
	case "head":
		// PowerShell fallback for head
		return nil // handled specially in shell_tool.go
	case "tail":
		return nil // handled specially in shell_tool.go
	case "du":
		return append([]string{"/C", "dir", "/s"}, args...)
	default:
		return args
	}
}
