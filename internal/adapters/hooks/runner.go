// Package hooks provides the FileSystemHookRunner — a HookRunnerPort adapter
// that discovers and executes hook scripts from the filesystem.
//
// # Discovery
//
// Hooks are loaded from two sources, merged in this order:
//  1. Config.toml [[hooks.<EventType>]] entries (explicit, highest precedence).
//  2. Convention-based directory: ~/.duckops/hooks/<EventType>/*.sh (auto-discovered).
//
// # Protocol
//
// Each hook receives a JSON-encoded HookInput on stdin. It must write either
// nothing or a JSON-encoded HookOutput to stdout. All debug output must go
// to stderr — any non-JSON text on stdout causes the hook to fail.
//
//	Exit code 0  → stdout parsed as JSON HookOutput (empty object = allow)
//	Exit code 2  → critical block; stderr used as the rejection reason
//	Other codes  → non-fatal warning; execution continues
//
// # Security
//
// Hook scripts are executed with the current user's privileges. Project-level
// hooks (in .agents/hooks/ or .duckops/hooks/) are treated as untrusted until
// fingerprinted. Global hooks in ~/.duckops/hooks/ are always trusted.
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/types"
)

const (
	defaultTimeoutMs      = 30_000
	exitCodeCriticalBlock = 2
)

// FileSystemHookRunner implements ports.HookRunnerPort.
// It is safe for concurrent use.
type FileSystemHookRunner struct {
	hooks  map[domain.HookEventType][]domain.HookConfig
	logger shared_ports.Logger
}

var _ ports.HookRunnerPort = (*FileSystemHookRunner)(nil)

// New creates a FileSystemHookRunner.
//
//   - configHooks: hooks loaded from config.toml (may be nil)
//   - hooksDir: base directory for convention-based discovery (e.g. ~/.duckops/hooks)
//   - logger: may be nil
func New(
	configHooks map[domain.HookEventType][]domain.HookConfig,
	hooksDir string,
	logger shared_ports.Logger,
) *FileSystemHookRunner {
	merged := make(map[domain.HookEventType][]domain.HookConfig)

	// 1. Start with config.toml entries
	for event, cfgs := range configHooks {
		for _, cfg := range cfgs {
			if cfg.Enabled {
				merged[event] = append(merged[event], cfg)
			}
		}
	}

	// 2. Discover convention-based scripts
	events := []domain.HookEventType{
		domain.HookBeforeTool,
		domain.HookAfterTool,
		domain.HookBeforeScan,
		domain.HookAfterScan,
		domain.HookSessionStart,
		domain.HookSessionEnd,
	}
	for _, event := range events {
		dir := filepath.Join(hooksDir, string(event))
		discovered := discoverScripts(dir)
		merged[event] = append(merged[event], discovered...)
	}

	return &FileSystemHookRunner{hooks: merged, logger: logger}
}

// discoverScripts finds all executable .sh (and on Windows .ps1) files in dir.
func discoverScripts(dir string) []domain.HookConfig {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // directory doesn't exist — fine
	}

	var result []domain.HookConfig
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sh") && !strings.HasSuffix(name, ".ps1") {
			continue
		}
		result = append(result, domain.HookConfig{
			Name:      strings.TrimSuffix(name, filepath.Ext(name)),
			Command:   filepath.Join(dir, name),
			TimeoutMs: defaultTimeoutMs,
			Enabled:   true,
		})
	}
	return result
}

// RunBeforeTool fires BeforeTool hooks. Returns deny if any hook blocks.
func (r *FileSystemHookRunner) RunBeforeTool(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error) {
	return r.run(ctx, domain.HookBeforeTool, input, true)
}

// RunAfterTool fires AfterTool hooks. Advisory — never returns deny.
func (r *FileSystemHookRunner) RunAfterTool(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error) {
	out, _ := r.run(ctx, domain.HookAfterTool, input, false)
	return out, nil
}

// RunBeforeScan fires BeforeScan hooks. Returns deny if any hook blocks.
func (r *FileSystemHookRunner) RunBeforeScan(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error) {
	return r.run(ctx, domain.HookBeforeScan, input, true)
}

// RunAfterScan fires AfterScan hooks. Advisory.
func (r *FileSystemHookRunner) RunAfterScan(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error) {
	out, _ := r.run(ctx, domain.HookAfterScan, input, false)
	return out, nil
}

// RunSessionStart fires SessionStart hooks. Advisory.
func (r *FileSystemHookRunner) RunSessionStart(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error) {
	out, _ := r.run(ctx, domain.HookSessionStart, input, false)
	return out, nil
}

// RunSessionEnd fires SessionEnd hooks. Advisory.
func (r *FileSystemHookRunner) RunSessionEnd(ctx context.Context, input domain.HookInput) (*domain.HookOutput, error) {
	out, _ := r.run(ctx, domain.HookSessionEnd, input, false)
	return out, nil
}

// List returns all registered hooks grouped by event type.
func (r *FileSystemHookRunner) List() map[domain.HookEventType][]domain.HookConfig {
	result := make(map[domain.HookEventType][]domain.HookConfig, len(r.hooks))
	for k, v := range r.hooks {
		cp := make([]domain.HookConfig, len(v))
		copy(cp, v)
		result[k] = cp
	}
	return result
}

// run executes all hooks for the given event and returns the aggregated decision.
// If canBlock is false, deny decisions are logged but never surfaced as errors.
func (r *FileSystemHookRunner) run(
	ctx context.Context,
	event domain.HookEventType,
	input domain.HookInput,
	canBlock bool,
) (*domain.HookOutput, error) {
	cfgs := r.hooks[event]
	if len(cfgs) == 0 {
		return nil, nil
	}

	input.Event = event
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "hooks: failed to marshal input")
	}

	for _, cfg := range cfgs {
		if !r.matches(cfg, input.ToolName) {
			continue
		}

		out, blocked, reason, execErr := r.execHook(ctx, cfg, inputJSON)
		if execErr != nil {
			r.logf(ctx, "hook %q (%s) failed: %v", cfg.Name, event, execErr)
			continue // non-fatal
		}

		if blocked && canBlock {
			return &domain.HookOutput{
				Decision: domain.HookDeny,
				Reason:   reason,
			}, types.Newf(types.ErrCodeSecurityViolation, "hook %q blocked execution: %s", cfg.Name, reason)
		}

		if out != nil && out.Decision == domain.HookDeny {
			if canBlock {
				return out, types.Newf(types.ErrCodeSecurityViolation, "hook %q denied: %s", cfg.Name, out.Reason)
			}
			r.logf(ctx, "hook %q returned deny (advisory, ignored): %s", cfg.Name, out.Reason)
		}

		// First non-empty system message wins
		if out != nil && out.SystemMessage != "" {
			return out, nil
		}
	}

	return nil, nil
}

// execHook runs a single hook script and returns its parsed output.
//   - blocked=true means exit code 2 (critical block)
//   - reason is the stderr content when blocked
func (r *FileSystemHookRunner) execHook(
	ctx context.Context,
	cfg domain.HookConfig,
	inputJSON []byte,
) (out *domain.HookOutput, blocked bool, reason string, err error) {
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = time.Duration(defaultTimeoutMs) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Expand environment variables in the command string
	command := os.ExpandEnv(cfg.Command)

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdin = bytes.NewReader(inputJSON)
	cmd.Env = append(os.Environ(),
		"DUCKOPS_SESSION_ID="+safeStr(inputJSON, "session_id"),
		"DUCKOPS_PROJECT_DIR="+safeStr(inputJSON, "project_dir"),
		"DUCKOPS_EVENT="+string(cfg.Name),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, false, "", types.Wrap(runErr, types.ErrCodeExecutionFailed, "hook exec failed")
		}
	}

	// Exit code 2 = critical block
	if exitCode == exitCodeCriticalBlock {
		return nil, true, strings.TrimSpace(stderr.String()), nil
	}

	// Other non-zero = warning
	if exitCode != 0 {
		r.logf(context.Background(), "hook %q exited with code %d (warning): %s",
			cfg.Name, exitCode, stderr.String())
		return nil, false, "", nil
	}

	// Exit code 0 — parse stdout as JSON
	rawOut := bytes.TrimSpace(stdout.Bytes())
	if len(rawOut) == 0 {
		return nil, false, "", nil // empty = allow
	}

	var hookOut domain.HookOutput
	if jsonErr := json.Unmarshal(rawOut, &hookOut); jsonErr != nil {
		r.logf(context.Background(), "hook %q stdout is not valid JSON: %v — treating as allow",
			cfg.Name, jsonErr)
		return nil, false, "", nil
	}

	return &hookOut, false, "", nil
}

// matches checks whether a hook should fire for the given tool name.
// Empty matcher = matches all tools.
func (r *FileSystemHookRunner) matches(cfg domain.HookConfig, toolName string) bool {
	if cfg.Matcher == "" {
		return true
	}
	re, err := regexp.Compile(cfg.Matcher)
	if err != nil {
		return false
	}
	return re.MatchString(toolName)
}

func (r *FileSystemHookRunner) logf(ctx context.Context, format string, args ...interface{}) {
	if r.logger == nil {
		return
	}
	r.logger.Debug(ctx, fmt.Sprintf(format, args...))
}

// safeStr extracts a string field from a JSON blob without full unmarshal.
// Returns empty string on any error — used only for environment variables.
func safeStr(data []byte, key string) string {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
