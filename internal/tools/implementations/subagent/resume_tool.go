package subagent

import (
	"context"
	"fmt"

	"github.com/SecDuckOps/agent/internal/domain"
	sa "github.com/SecDuckOps/agent/internal/domain/subagent"
	tracker "github.com/SecDuckOps/agent/internal/subagent"
	"github.com/SecDuckOps/agent/internal/tools/base"
)

// ResumeParams defines the input for resuming a paused subagent.
type ResumeParams struct {
	TaskID     string   `json:"task_id"`
	Approve    []string `json:"approve,omitempty"`
	Reject     []string `json:"reject,omitempty"`
	ApproveAll bool     `json:"approve_all,omitempty"`
	RejectAll  bool     `json:"reject_all,omitempty"`
	Input      string   `json:"input,omitempty"`
}

// ResumeTool resumes a paused subagent with approval decisions.
type ResumeTool struct {
	base.BaseTypedTool[ResumeParams]
	tracker *tracker.Tracker
}

func NewResumeTool(t *tracker.Tracker) *ResumeTool {
	tool := &ResumeTool{tracker: t}
	tool.Impl = tool
	return tool
}

func (t *ResumeTool) Name() string { return "resume_subagent_task" }

func (t *ResumeTool) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "resume_subagent_task",
		Description: `Resume a paused subagent task with approval decisions or follow-up input.

WORKFLOW:
1. Start subagent: dynamic_subagent_task (non-sandbox subagents auto-pause on tool calls)
2. Monitor with session status — check for status 'paused'
3. Read pause_info to see pending_tool_calls
4. Resume with approve/reject decisions
5. The subagent continues execution from where it stopped

PARAMETERS:
- task_id: Session ID of the paused subagent
- approve: List of tool call IDs to approve (e.g., ["tc_abc123_0"])
- reject: List of tool call IDs to reject
- approve_all: Approve all pending tool calls
- reject_all: Reject all pending tool calls
- input: Text input for follow-up (for input_required pauses)

NOTES:
- Only works on sessions with status 'paused'
- Unspecified tool calls are rejected by default
- Sandbox subagents never pause (they run autonomously)`,
		Parameters: map[string]string{
			"task_id":     "string (required) - Session ID of the paused subagent",
			"approve":     "[]string (optional) - Tool call IDs to approve",
			"reject":      "[]string (optional) - Tool call IDs to reject",
			"approve_all": "bool (optional) - Approve all pending tool calls",
			"reject_all":  "bool (optional) - Reject all pending tool calls",
			"input":       "string (optional) - Text input for input_required pauses",
		},
	}
}

func (t *ResumeTool) ParseParams(input map[string]interface{}) (ResumeParams, error) {
	return base.DefaultParseParams[ResumeParams](input)
}

func (t *ResumeTool) Execute(ctx context.Context, params ResumeParams) (domain.Result, error) {
	if params.TaskID == "" {
		return domain.Result{Success: false, Error: "task_id is required"}, nil
	}

	decision := sa.ResumeDecision{
		Approve:    params.Approve,
		Reject:     params.Reject,
		ApproveAll: params.ApproveAll,
		RejectAll:  params.RejectAll,
		Input:      params.Input,
	}

	err := t.tracker.ResumeSession(params.TaskID, decision)
	if err != nil {
		return domain.Result{
			Success: false,
			Error:   fmt.Sprintf("Failed to resume: %v", err),
		}, nil
	}

	return domain.Result{
		Success: true,
		Status:  "resumed",
		Data: map[string]interface{}{
			"task_id": params.TaskID,
			"message": "Subagent resumed successfully",
		},
	}, nil
}
