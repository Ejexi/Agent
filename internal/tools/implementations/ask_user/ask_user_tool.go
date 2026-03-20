// Package ask_user provides a tool that lets the LLM pause execution and ask
// the user structured questions (multiple-choice, text, yes/no).
//
// The tool sends an AskUserEvent through the kernel's event callback, then
// blocks on a response channel until the TUI delivers the user's answer.
// This creates a synchronous bridge between the async TUI and the blocking tool.
//
// Timeout: if no answer arrives within 10 minutes, the tool returns a
// cancelled response so the agent can continue gracefully.
package ask_user

import (
	"context"
	"encoding/json"
	"time"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/types"
)

const userResponseTimeout = 10 * time.Minute

// AskUserParams are the parameters passed by the LLM.
type AskUserParams struct {
	Questions []questionInput `json:"questions"`
}

type questionInput struct {
	Header      string                        `json:"header"`
	Question    string                        `json:"question"`
	Type        agent_domain.AskUserQuestionType `json:"type"`
	Options     []optionInput                 `json:"options,omitempty"`
	MultiSelect bool                          `json:"multi_select,omitempty"`
	Placeholder string                        `json:"placeholder,omitempty"`
}

type optionInput struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// AskUserTool pauses the agent loop and collects structured answers from the user.
type AskUserTool struct {
	base.BaseTypedTool[AskUserParams]
}

func New() *AskUserTool {
	t := &AskUserTool{}
	t.Impl = t
	return t
}

func (t *AskUserTool) Name() string { return "ask_user" }

func (t *AskUserTool) Schema() agent_domain.ToolSchema {
	return agent_domain.ToolSchema{
		Name: "ask_user",
		Description: `Ask the user one or more structured questions and wait for their answers.
Use this when you need a decision, preference, or clarification before proceeding.
Supports three question types:
- "choice": present 2-4 options (supports multi_select)
- "text":   free-form text input
- "yesno":  simple yes/no confirmation

Maximum 4 questions per call. Keep headers short (≤16 chars).`,
		Parameters: map[string]string{
			"questions": `array (required, max 4) of question objects:
  - header      string (required, ≤16 chars): short label shown as a chip
  - question    string (required): the full question text
  - type        string (required): "choice" | "text" | "yesno"
  - options     array (required for "choice"): [{label, description}, ...], 2-4 items
  - multi_select bool (optional, "choice" only): allow multiple selections
  - placeholder  string (optional, "text" only): hint shown in input field`,
		},
	}
}

func (t *AskUserTool) ParseParams(input map[string]interface{}) (AskUserParams, error) {
	return base.DefaultParseParams[AskUserParams](input)
}

func (t *AskUserTool) Execute(ctx context.Context, params AskUserParams) (agent_domain.Result, error) {
	if len(params.Questions) == 0 {
		return agent_domain.Result{
			Success: false,
			Error:   types.New(types.ErrCodeInvalidInput, "ask_user: at least one question is required").Error(),
		}, nil
	}
	if len(params.Questions) > 4 {
		params.Questions = params.Questions[:4]
	}

	// Build the domain event
	questions := make([]agent_domain.AskUserQuestion, len(params.Questions))
	for i, q := range params.Questions {
		qt := q.Type
		if qt == "" {
			qt = agent_domain.QuestionTypeChoice
		}
		opts := make([]agent_domain.AskUserOption, len(q.Options))
		for j, o := range q.Options {
			opts[j] = agent_domain.AskUserOption{Label: o.Label, Description: o.Description}
		}
		questions[i] = agent_domain.AskUserQuestion{
			Header:      q.Header,
			Question:    q.Question,
			Type:        qt,
			Options:     opts,
			MultiSelect: q.MultiSelect,
			Placeholder: q.Placeholder,
		}
	}

	responseChan := make(chan agent_domain.AskUserResponse, 1)
	event := agent_domain.AskUserEvent{
		Questions:    questions,
		ResponseChan: responseChan,
	}

	// Send event through the kernel execution context callback.
	// The TUI's event handler picks this up and shows the dialog.
	if execCtx, ok := kernel.FromContext(ctx); ok {
		execCtx.Emit(event)
	} else {
		// Headless / non-TUI context — return a sensible fallback
		return agent_domain.Result{
			Success: true,
			Data: map[string]interface{}{
				"answers":   map[string]string{},
				"cancelled": true,
				"note":      "ask_user is only interactive in TUI mode",
			},
		}, nil
	}

	// Block until user answers or timeout
	select {
	case resp := <-responseChan:
		if resp.Cancelled {
			return agent_domain.Result{
				Success: true,
				Data: map[string]interface{}{
					"cancelled": true,
					"answers":   map[string]string{},
				},
			}, nil
		}

		// Serialise answers as JSON string for the LLM
		answersJSON, _ := json.Marshal(resp.Answers)
		return agent_domain.Result{
			Success: true,
			Data: map[string]interface{}{
				"answers":   resp.Answers,
				"summary":   string(answersJSON),
				"cancelled": false,
			},
		}, nil

	case <-time.After(userResponseTimeout):
		return agent_domain.Result{
			Success: true,
			Data: map[string]interface{}{
				"cancelled": true,
				"answers":   map[string]string{},
				"note":      "user did not respond within timeout",
			},
		}, nil

	case <-ctx.Done():
		return agent_domain.Result{
			Success: false,
			Error:   types.New(types.ErrCodeInternal, "ask_user: context cancelled").Error(),
		}, nil
	}
}
