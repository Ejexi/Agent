package domain

// AskUserQuestionType defines the input type for a question.
type AskUserQuestionType string

const (
	QuestionTypeChoice AskUserQuestionType = "choice"
	QuestionTypeText   AskUserQuestionType = "text"
	QuestionTypeYesNo  AskUserQuestionType = "yesno"
)

// AskUserOption is a single selectable option in a choice question.
type AskUserOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// AskUserQuestion is a single question presented to the user.
type AskUserQuestion struct {
	Header      string              `json:"header"`
	Question    string              `json:"question"`
	Type        AskUserQuestionType `json:"type"`
	Options     []AskUserOption     `json:"options,omitempty"`
	MultiSelect bool                `json:"multi_select,omitempty"`
	Placeholder string              `json:"placeholder,omitempty"`
}

// AskUserEvent is sent through the engine event channel when the agent needs
// user input. The tool blocks on ResponseChan until the TUI sends an answer.
type AskUserEvent struct {
	Questions    []AskUserQuestion
	ResponseChan chan AskUserResponse
}

// AskUserResponse carries the user's answers back to the tool.
type AskUserResponse struct {
	// Answers maps question index (0-based) to the user's answer string.
	// For choice questions with multi-select, values are comma-separated.
	// For yesno, value is "yes" or "no".
	Answers map[int]string
	// Cancelled is true if the user dismissed the dialog without answering.
	Cancelled bool
}

// PlanModeChangedEvent is emitted when the agent enters or exits plan mode.
// The TUI uses this to show/hide the "PLAN MODE" indicator in the header.
type PlanModeChangedEvent struct {
	Active bool
	Reason string
}

// SkillActivationEvent is emitted by load_skill before injecting skill content.
// The TUI shows a consent dialog; if the user denies, the skill is not loaded.
type SkillActivationEvent struct {
	SkillName    string
	Description  string
	SkillDir     string // path the skill will gain read access to
	ResponseChan chan bool // true = approved, false = denied
}
