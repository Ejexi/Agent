// Package plan provides enter_plan_mode and exit_plan_mode tools.
//
// These tools let the LLM explicitly enter a read-only research/planning phase
// before executing any scan. When in plan mode:
//   - The orchestrator runs Phase 1 (Understand) + Phase 2 (Plan)
//   - The plan is saved as a Markdown file in ~/.duckops/plans/
//   - No scanners run until the user approves via exit_plan_mode
//
// Flow:
//
//	User: "plan a security review of ./backend"
//	LLM calls enter_plan_mode{reason: "..."}
//	  → AOrchestra Phase 1+2 run
//	  → Plan written to ~/.duckops/plans/<timestamp>.md
//	  → PlanReadyEvent sent to TUI (shows plan, asks for approval)
//	User approves
//	LLM calls exit_plan_mode{plan_path: "..."}
//	  → Phase 3 (Execute) runs
package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/types"
)

// ─────────────────────────────────────────────────────────────────────────────
// PlanReadyEvent is emitted when a plan is written and ready for user approval.
// The TUI shows the plan contents and a yes/no confirmation.
// ─────────────────────────────────────────────────────────────────────────────

// PlanReadyEvent is sent through the kernel event callback when a plan file is
// ready for user review. The tool blocks on ResponseChan until the TUI sends
// the approval decision.
type PlanReadyEvent struct {
	PlanPath     string
	PlanMarkdown string
	ResponseChan chan PlanApproval
}

// PlanApproval is the user's decision after reviewing the plan.
type PlanApproval struct {
	Approved bool
	Feedback string // non-empty when Approved=false — returned to LLM for iteration
}

// ─────────────────────────────────────────────────────────────────────────────
// enter_plan_mode
// ─────────────────────────────────────────────────────────────────────────────

// EnterPlanModeParams are the parameters for enter_plan_mode.
type EnterPlanModeParams struct {
	Reason string `json:"reason,omitempty"`
}

// EnterPlanModeTool switches the session into planning mode.
// Called by the LLM when the user asks to plan before scanning.
type EnterPlanModeTool struct {
	base.BaseTypedTool[EnterPlanModeParams]
}

func NewEnterPlanMode() *EnterPlanModeTool {
	t := &EnterPlanModeTool{}
	t.Impl = t
	return t
}

func (t *EnterPlanModeTool) Name() string { return "enter_plan_mode" }

func (t *EnterPlanModeTool) Schema() agent_domain.ToolSchema {
	return agent_domain.ToolSchema{
		Name: "enter_plan_mode",
		Description: `Switch to plan mode to research and design a security review before executing any scans.
Use this when the user asks to "plan", "design", or "think through" a scan approach.
In plan mode you may read files and analyse the project but must NOT run any scanners.
After planning, call exit_plan_mode with the path to the written plan file.`,
		Parameters: map[string]string{
			"reason": "string (optional): brief explanation of why you are entering plan mode",
		},
	}
}

func (t *EnterPlanModeTool) ParseParams(input map[string]interface{}) (EnterPlanModeParams, error) {
	return base.DefaultParseParams[EnterPlanModeParams](input)
}

func (t *EnterPlanModeTool) Execute(ctx context.Context, params EnterPlanModeParams) (agent_domain.Result, error) {
	reason := params.Reason
	if reason == "" {
		reason = "preparing scan strategy"
	}

	// Emit event to TUI so it can show a "Plan mode active" indicator
	if execCtx, ok := ctx.(*kernel.ExecutionContext); ok {
		execCtx.Emit(agent_domain.PlanModeChangedEvent{Active: true, Reason: reason})
	}

	return agent_domain.Result{
		Success: true,
		Data: map[string]interface{}{
			"mode":    "plan",
			"message": fmt.Sprintf("Switched to plan mode: %s. Analyse the project, then write a plan and call exit_plan_mode.", reason),
		},
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// exit_plan_mode
// ─────────────────────────────────────────────────────────────────────────────

// ExitPlanModeParams are the parameters for exit_plan_mode.
type ExitPlanModeParams struct {
	// PlanMarkdown is the complete Markdown plan text.
	// The tool writes it to ~/.duckops/plans/ automatically.
	PlanMarkdown string `json:"plan_markdown"`
	// Summary is a one-line summary shown in the TUI approval dialog.
	Summary string `json:"summary,omitempty"`
}

// ExitPlanModeTool presents the plan to the user for approval and then signals
// that execution should proceed.
type ExitPlanModeTool struct {
	base.BaseTypedTool[ExitPlanModeParams]
}

func NewExitPlanMode() *ExitPlanModeTool {
	t := &ExitPlanModeTool{}
	t.Impl = t
	return t
}

func (t *ExitPlanModeTool) Name() string { return "exit_plan_mode" }

func (t *ExitPlanModeTool) Schema() agent_domain.ToolSchema {
	return agent_domain.ToolSchema{
		Name: "exit_plan_mode",
		Description: `Finalise the plan and present it to the user for approval.
Write the complete Markdown plan in plan_markdown. The tool saves it to disk,
shows it to the user, and waits for their approval or feedback.
If the user approves, proceed with scan execution.
If the user provides feedback, iterate on the plan and call exit_plan_mode again.`,
		Parameters: map[string]string{
			"plan_markdown": "string (required): the complete Markdown plan text",
			"summary":       "string (optional): one-line summary shown in the approval dialog",
		},
	}
}

func (t *ExitPlanModeTool) ParseParams(input map[string]interface{}) (ExitPlanModeParams, error) {
	return base.DefaultParseParams[ExitPlanModeParams](input)
}

func (t *ExitPlanModeTool) Execute(ctx context.Context, params ExitPlanModeParams) (agent_domain.Result, error) {
	if params.PlanMarkdown == "" {
		return agent_domain.Result{
			Success: false,
			Error:   types.New(types.ErrCodeInvalidInput, "exit_plan_mode: plan_markdown is required").Error(),
		}, nil
	}

	// Write plan to ~/.duckops/plans/<timestamp>.md
	planPath, err := writePlan(params.PlanMarkdown)
	if err != nil {
		return agent_domain.Result{
			Success: false,
			Error:   types.Wrap(err, types.ErrCodeInternal, "failed to write plan file").Error(),
		}, nil
	}

	// Send PlanReadyEvent — TUI shows the plan and asks for approval
	responseChan := make(chan PlanApproval, 1)
	event := PlanReadyEvent{
		PlanPath:     planPath,
		PlanMarkdown: params.PlanMarkdown,
		ResponseChan: responseChan,
	}

	if execCtx, ok := ctx.(*kernel.ExecutionContext); ok {
		execCtx.Emit(event)
	} else {
		// Headless — auto-approve
		return agent_domain.Result{
			Success: true,
			Data: map[string]interface{}{
				"approved":  true,
				"plan_path": planPath,
				"message":   "Plan written (headless mode — auto-approved). Proceeding with execution.",
			},
		}, nil
	}

	// Block until user approves or provides feedback (10-minute timeout)
	select {
	case approval := <-responseChan:
		if approval.Approved {
			// Signal plan mode exit
			if execCtx, ok := ctx.(*kernel.ExecutionContext); ok {
				execCtx.Emit(agent_domain.PlanModeChangedEvent{Active: false})
			}
			return agent_domain.Result{
				Success: true,
				Data: map[string]interface{}{
					"approved":  true,
					"plan_path": planPath,
					"message":   "Plan approved. Proceed with scan execution.",
				},
			}, nil
		}
		// Rejected — return feedback to LLM for iteration
		return agent_domain.Result{
			Success: true,
			Data: map[string]interface{}{
				"approved":  false,
				"feedback":  approval.Feedback,
				"plan_path": planPath,
				"message":   fmt.Sprintf("Plan rejected. User feedback: %s. Please revise and call exit_plan_mode again.", approval.Feedback),
			},
		}, nil

	case <-time.After(10 * time.Minute):
		return agent_domain.Result{
			Success: true,
			Data: map[string]interface{}{
				"approved":  false,
				"feedback":  "timeout",
				"plan_path": planPath,
				"message":   "User did not respond within timeout.",
			},
		}, nil

	case <-ctx.Done():
		return agent_domain.Result{
			Success: false,
			Error:   types.New(types.ErrCodeInternal, "exit_plan_mode: context cancelled").Error(),
		}, nil
	}
}

// writePlan saves the plan markdown to ~/.duckops/plans/ and returns its path.
func writePlan(markdown string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	plansDir := filepath.Join(home, ".duckops", "plans")
	if err := os.MkdirAll(plansDir, 0o700); err != nil {
		return "", err
	}
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	path := filepath.Join(plansDir, fmt.Sprintf("plan-%s.md", timestamp))
	if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// planSummaryJSON serialises the plan struct to a compact JSON string for the LLM.
func planSummaryJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
