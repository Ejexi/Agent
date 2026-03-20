package taskengine_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SecDuckOps/agent/internal/application/taskengine"
	"github.com/SecDuckOps/agent/internal/domain"
)

// ── stub HookRunner ───────────────────────────────────────────────────────────

type stubHookRunner struct {
	beforeDecision *domain.HookOutput
	beforeErr      error
	afterCalled    bool
}

func (s *stubHookRunner) RunBeforeTool(_ context.Context, _ domain.HookInput) (*domain.HookOutput, error) {
	return s.beforeDecision, s.beforeErr
}
func (s *stubHookRunner) RunAfterTool(_ context.Context, _ domain.HookInput) (*domain.HookOutput, error) {
	s.afterCalled = true
	return nil, nil
}
func (s *stubHookRunner) RunBeforeScan(_ context.Context, _ domain.HookInput) (*domain.HookOutput, error) {
	return nil, nil
}
func (s *stubHookRunner) RunAfterScan(_ context.Context, _ domain.HookInput) (*domain.HookOutput, error) {
	return nil, nil
}
func (s *stubHookRunner) RunSessionStart(_ context.Context, _ domain.HookInput) (*domain.HookOutput, error) {
	return nil, nil
}
func (s *stubHookRunner) RunSessionEnd(_ context.Context, _ domain.HookInput) (*domain.HookOutput, error) {
	return nil, nil
}
func (s *stubHookRunner) List() map[domain.HookEventType][]domain.HookConfig {
	return nil
}

// ── base handler that records whether it ran ──────────────────────────────────

func baseHandler(ran *bool) taskengine.TaskHandler {
	return func(_ context.Context, task *domain.OSTask) domain.OSTaskResult {
		*ran = true
		return domain.OSTaskResult{Status: domain.StatusCompleted, Stdout: "ok"}
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestHookMiddleware_NilRunner_IsTransparent(t *testing.T) {
	ran := false
	mw := taskengine.HookMiddleware(nil)
	handler := mw(baseHandler(&ran))

	result := handler(context.Background(), &domain.OSTask{OriginalCmd: "ls"})
	if !ran {
		t.Fatal("base handler should have run when runner is nil")
	}
	if result.Status != domain.StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
}

func TestHookMiddleware_AllowContinues(t *testing.T) {
	ran := false
	stub := &stubHookRunner{beforeDecision: nil, beforeErr: nil}
	mw := taskengine.HookMiddleware(stub)
	handler := mw(baseHandler(&ran))

	result := handler(context.Background(), &domain.OSTask{OriginalCmd: "ls"})
	if !ran {
		t.Fatal("base handler should have run when hook allows")
	}
	if result.Status != domain.StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
}

func TestHookMiddleware_DenyBlocksExecution(t *testing.T) {
	ran := false
	stub := &stubHookRunner{
		beforeDecision: &domain.HookOutput{Decision: domain.HookDeny, Reason: "blocked by policy"},
		beforeErr:      errors.New("blocked by policy"),
	}
	mw := taskengine.HookMiddleware(stub)
	handler := mw(baseHandler(&ran))

	result := handler(context.Background(), &domain.OSTask{OriginalCmd: "rm"})
	if ran {
		t.Fatal("base handler must NOT run when hook denies")
	}
	if result.Status != domain.StatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected non-nil error when hook denies")
	}
}

func TestHookMiddleware_AfterToolAlwaysRuns(t *testing.T) {
	stub := &stubHookRunner{}
	ran := false
	mw := taskengine.HookMiddleware(stub)
	handler := mw(baseHandler(&ran))

	handler(context.Background(), &domain.OSTask{OriginalCmd: "ls"})
	if !stub.afterCalled {
		t.Fatal("AfterTool hook should always be called after execution")
	}
}

func TestHookMiddleware_AfterToolRunsEvenOnFailure(t *testing.T) {
	// Base handler returns a failure
	stub := &stubHookRunner{}
	mw := taskengine.HookMiddleware(stub)
	failHandler := mw(func(_ context.Context, _ *domain.OSTask) domain.OSTaskResult {
		return domain.OSTaskResult{Status: domain.StatusFailed, Error: errors.New("cmd failed")}
	})

	failHandler(context.Background(), &domain.OSTask{OriginalCmd: "bad-cmd"})
	if !stub.afterCalled {
		t.Fatal("AfterTool must run even when the base handler fails")
	}
}
