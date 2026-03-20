package hooks_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	hooks_adapter "github.com/SecDuckOps/agent/internal/adapters/hooks"
	"github.com/SecDuckOps/agent/internal/domain"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func script(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func newRunner(t *testing.T, cfgHooks map[domain.HookEventType][]domain.HookConfig) *hooks_adapter.FileSystemHookRunner {
	t.Helper()
	return hooks_adapter.New(cfgHooks, t.TempDir(), nil)
}

// ── allow / deny via exit code ────────────────────────────────────────────────

func TestRunBeforeTool_AllowEmptyOutput(t *testing.T) {
	dir := t.TempDir()
	cmd := script(t, dir, "allow.sh", `echo '{}'`)

	runner := newRunner(t, map[domain.HookEventType][]domain.HookConfig{
		domain.HookBeforeTool: {{Name: "allow", Command: cmd, Enabled: true}},
	})

	out, err := runner.RunBeforeTool(context.Background(), domain.HookInput{ToolName: "shell"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != nil && out.Decision == domain.HookDeny {
		t.Fatalf("expected allow, got deny")
	}
}

func TestRunBeforeTool_DenyViaJSON(t *testing.T) {
	dir := t.TempDir()
	cmd := script(t, dir, "deny.sh", `echo '{"decision":"deny","reason":"not allowed"}'`)

	runner := newRunner(t, map[domain.HookEventType][]domain.HookConfig{
		domain.HookBeforeTool: {{Name: "block", Command: cmd, Enabled: true}},
	})

	out, err := runner.RunBeforeTool(context.Background(), domain.HookInput{ToolName: "shell"})
	if err == nil {
		t.Fatal("expected error from deny decision, got nil")
	}
	if out == nil || out.Decision != domain.HookDeny {
		t.Fatalf("expected deny output, got %+v", out)
	}
	if out.Reason != "not allowed" {
		t.Fatalf("expected reason 'not allowed', got %q", out.Reason)
	}
}

func TestRunBeforeTool_DenyViaExitCode2(t *testing.T) {
	dir := t.TempDir()
	// exit code 2 = critical block; reason taken from stderr
	cmd := script(t, dir, "exit2.sh", `echo "production scan blocked" >&2; exit 2`)

	runner := newRunner(t, map[domain.HookEventType][]domain.HookConfig{
		domain.HookBeforeTool: {{Name: "hard-block", Command: cmd, Enabled: true}},
	})

	_, err := runner.RunBeforeTool(context.Background(), domain.HookInput{ToolName: "scan"})
	if err == nil {
		t.Fatal("expected error from exit code 2, got nil")
	}
}

// ── AfterTool is advisory: deny never returns an error ────────────────────────

func TestRunAfterTool_DenyIsAdvisoryOnly(t *testing.T) {
	dir := t.TempDir()
	cmd := script(t, dir, "after-deny.sh", `echo '{"decision":"deny","reason":"too late"}'`)

	runner := newRunner(t, map[domain.HookEventType][]domain.HookConfig{
		domain.HookAfterTool: {{Name: "advisory", Command: cmd, Enabled: true}},
	})

	_, err := runner.RunAfterTool(context.Background(), domain.HookInput{ToolName: "shell"})
	if err != nil {
		t.Fatalf("AfterTool deny must be advisory (no error), got %v", err)
	}
}

// ── matcher filtering ─────────────────────────────────────────────────────────

func TestRunBeforeTool_MatcherFiltersCorrectly(t *testing.T) {
	dir := t.TempDir()
	// This hook should only fire for "shell" or "terminal"
	blocked := false
	cmd := script(t, dir, "block-shell.sh", `echo '{"decision":"deny","reason":"shell blocked"}'`)

	runner := newRunner(t, map[domain.HookEventType][]domain.HookConfig{
		domain.HookBeforeTool: {{
			Name:    "block-shell",
			Matcher: "^(shell|terminal)$",
			Command: cmd,
			Enabled: true,
		}},
	})

	// Should NOT match "web_search"
	_, err := runner.RunBeforeTool(context.Background(), domain.HookInput{ToolName: "web_search"})
	if err != nil {
		blocked = true
	}
	if blocked {
		t.Fatal("matcher should not have fired for 'web_search'")
	}

	// SHOULD match "shell"
	_, err = runner.RunBeforeTool(context.Background(), domain.HookInput{ToolName: "shell"})
	if err == nil {
		t.Fatal("matcher should have fired for 'shell'")
	}
}

// ── no hooks configured → nil, nil ───────────────────────────────────────────

func TestRunBeforeScan_NoHooksConfigured(t *testing.T) {
	runner := newRunner(t, nil)
	out, err := runner.RunBeforeScan(context.Background(), domain.HookInput{ScanTarget: "/tmp"})
	if err != nil {
		t.Fatalf("expected no error with no hooks, got %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil output with no hooks, got %+v", out)
	}
}

// ── convention-based directory discovery ─────────────────────────────────────

func TestDiscovery_ConventionDir(t *testing.T) {
	hooksDir := t.TempDir()
	eventDir := filepath.Join(hooksDir, "BeforeScan")
	if err := os.MkdirAll(eventDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a script that denies
	script(t, eventDir, "block.sh", `echo '{"decision":"deny","reason":"convention hook"}'`)

	runner := hooks_adapter.New(nil, hooksDir, nil)

	_, err := runner.RunBeforeScan(context.Background(), domain.HookInput{ScanTarget: "/tmp"})
	if err == nil {
		t.Fatal("convention-discovered hook should have blocked scan")
	}
}

// ── system message passthrough ────────────────────────────────────────────────

func TestRunAfterScan_SystemMessage(t *testing.T) {
	dir := t.TempDir()
	cmd := script(t, dir, "msg.sh", `echo '{"system_message":"scan complete, 3 findings"}'`)

	runner := newRunner(t, map[domain.HookEventType][]domain.HookConfig{
		domain.HookAfterScan: {{Name: "notify", Command: cmd, Enabled: true}},
	})

	out, err := runner.RunAfterScan(context.Background(), domain.HookInput{ScanTarget: "/tmp", Findings: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil || out.SystemMessage != "scan complete, 3 findings" {
		t.Fatalf("expected system message, got %+v", out)
	}
}

// ── disabled hook is skipped ──────────────────────────────────────────────────

func TestDisabledHookIsSkipped(t *testing.T) {
	dir := t.TempDir()
	cmd := script(t, dir, "deny.sh", `echo '{"decision":"deny","reason":"should not fire"}'`)

	runner := newRunner(t, map[domain.HookEventType][]domain.HookConfig{
		domain.HookBeforeTool: {{Name: "disabled", Command: cmd, Enabled: false}},
	})

	_, err := runner.RunBeforeTool(context.Background(), domain.HookInput{ToolName: "shell"})
	if err != nil {
		t.Fatalf("disabled hook should not fire, got error: %v", err)
	}
}

// ── non-fatal exit code (e.g. 1) is a warning, not a block ───────────────────

func TestNonFatalExitCode_IsWarning(t *testing.T) {
	dir := t.TempDir()
	cmd := script(t, dir, "warn.sh", `exit 1`)

	runner := newRunner(t, map[domain.HookEventType][]domain.HookConfig{
		domain.HookBeforeTool: {{Name: "warning-hook", Command: cmd, Enabled: true}},
	})

	_, err := runner.RunBeforeTool(context.Background(), domain.HookInput{ToolName: "shell"})
	if err != nil {
		t.Fatalf("non-fatal exit code should not block, got error: %v", err)
	}
}

// ── List returns registered hooks ────────────────────────────────────────────

func TestList_ReturnsAllRegistered(t *testing.T) {
	dir := t.TempDir()
	cmd := script(t, dir, "noop.sh", `echo '{}'`)

	runner := newRunner(t, map[domain.HookEventType][]domain.HookConfig{
		domain.HookBeforeScan: {{Name: "before", Command: cmd, Enabled: true}},
		domain.HookAfterScan:  {{Name: "after", Command: cmd, Enabled: true}},
	})

	listed := runner.List()
	if len(listed[domain.HookBeforeScan]) != 1 {
		t.Fatalf("expected 1 BeforeScan hook, got %d", len(listed[domain.HookBeforeScan]))
	}
	if len(listed[domain.HookAfterScan]) != 1 {
		t.Fatalf("expected 1 AfterScan hook, got %d", len(listed[domain.HookAfterScan]))
	}
}
