package components

import (
	"fmt"
	"strings"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

// AskUserDialog renders a structured question dialog in the TUI.
// Supports choice (single/multi), text, and yesno question types.
// Navigation: Tab/↑↓ move between questions and options, Enter confirms.
type AskUserDialog struct {
	Questions []agent_domain.AskUserQuestion

	// cursor[i] is the selected option index for question i (choice/yesno)
	cursors []int
	// multiSelected[i] is the set of selected indices for multi-select questions
	multiSelected []map[int]bool
	// textValues[i] is the current text input for text questions
	textValues []string
	// focusedQ is which question is currently focused
	focusedQ int
	// textCursor is the cursor position inside a text input
	textCursor int

	Width int
}

// NewAskUserDialog creates a dialog pre-populated with the given questions.
func NewAskUserDialog(questions []agent_domain.AskUserQuestion, width int) *AskUserDialog {
	d := &AskUserDialog{
		Questions:     questions,
		cursors:       make([]int, len(questions)),
		multiSelected: make([]map[int]bool, len(questions)),
		textValues:    make([]string, len(questions)),
		Width:         width,
	}
	for i := range questions {
		d.multiSelected[i] = make(map[int]bool)
		if questions[i].Type == agent_domain.QuestionTypeYesNo {
			d.cursors[i] = 0 // default: Yes
		}
	}
	return d
}

// MoveUp moves the cursor up within the focused question.
func (d *AskUserDialog) MoveUp() {
	q := d.Questions[d.focusedQ]
	if q.Type == agent_domain.QuestionTypeChoice && d.cursors[d.focusedQ] > 0 {
		d.cursors[d.focusedQ]--
	}
	if q.Type == agent_domain.QuestionTypeYesNo && d.cursors[d.focusedQ] > 0 {
		d.cursors[d.focusedQ]--
	}
}

// MoveDown moves the cursor down within the focused question.
func (d *AskUserDialog) MoveDown() {
	q := d.Questions[d.focusedQ]
	if q.Type == agent_domain.QuestionTypeChoice && d.cursors[d.focusedQ] < len(q.Options)-1 {
		d.cursors[d.focusedQ]++
	}
	if q.Type == agent_domain.QuestionTypeYesNo && d.cursors[d.focusedQ] < 1 {
		d.cursors[d.focusedQ]++
	}
}

// NextQuestion moves focus to the next question.
func (d *AskUserDialog) NextQuestion() {
	if d.focusedQ < len(d.Questions)-1 {
		d.focusedQ++
	}
}

// PrevQuestion moves focus to the previous question.
func (d *AskUserDialog) PrevQuestion() {
	if d.focusedQ > 0 {
		d.focusedQ--
	}
}

// ToggleMultiSelect toggles the currently highlighted option in a multi-select question.
func (d *AskUserDialog) ToggleMultiSelect() {
	q := d.Questions[d.focusedQ]
	if q.Type == agent_domain.QuestionTypeChoice && q.MultiSelect {
		idx := d.cursors[d.focusedQ]
		if d.multiSelected[d.focusedQ][idx] {
			delete(d.multiSelected[d.focusedQ], idx)
		} else {
			d.multiSelected[d.focusedQ][idx] = true
		}
	}
}

// TypeChar appends a character to the current text input.
func (d *AskUserDialog) TypeChar(ch rune) {
	q := d.Questions[d.focusedQ]
	if q.Type == agent_domain.QuestionTypeText {
		v := d.textValues[d.focusedQ]
		d.textValues[d.focusedQ] = v + string(ch)
	}
}

// Backspace removes the last character from the current text input.
func (d *AskUserDialog) Backspace() {
	q := d.Questions[d.focusedQ]
	if q.Type == agent_domain.QuestionTypeText {
		v := []rune(d.textValues[d.focusedQ])
		if len(v) > 0 {
			d.textValues[d.focusedQ] = string(v[:len(v)-1])
		}
	}
}

// Collect returns the answers for all questions as a map[int]string.
func (d *AskUserDialog) Collect() map[int]string {
	answers := make(map[int]string, len(d.Questions))
	for i, q := range d.Questions {
		switch q.Type {
		case agent_domain.QuestionTypeChoice:
			if q.MultiSelect {
				var labels []string
				for j, opt := range q.Options {
					if d.multiSelected[i][j] {
						labels = append(labels, opt.Label)
					}
				}
				answers[i] = strings.Join(labels, ", ")
			} else {
				if d.cursors[i] < len(q.Options) {
					answers[i] = q.Options[d.cursors[i]].Label
				}
			}
		case agent_domain.QuestionTypeYesNo:
			if d.cursors[i] == 0 {
				answers[i] = "yes"
			} else {
				answers[i] = "no"
			}
		case agent_domain.QuestionTypeText:
			answers[i] = d.textValues[i]
		}
	}
	return answers
}

// SelectedIsYes returns true if the selected option for the given question index is "yes".
func (d *AskUserDialog) SelectedIsYes(qIndex int) bool {
	if qIndex >= 0 && qIndex < len(d.cursors) {
		return d.cursors[qIndex] == 0
	}
	return true
}

// View renders the full dialog.
func (d *AskUserDialog) View() string {
	w := d.Width - 8
	if w < 44 {
		w = 44
	}

	if len(d.Questions) == 1 && d.Questions[0].Type == agent_domain.QuestionTypeYesNo {
		return d.CompactView(w)
	}

	borderColor := lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
	headerColor := lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	mutedColor := lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	accentColor := lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#5B9BD5"}
	selectedBg := lipgloss.AdaptiveColor{Light: "#EEF2FF", Dark: "#2A2D3E"}
	checkColor := lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#5B9BD5"}

	title := lipgloss.NewStyle().Bold(true).Foreground(headerColor).
		Render("🦆  DuckOps needs your input")
	hint := lipgloss.NewStyle().Foreground(mutedColor).
		Render("↑↓ navigate  ·  Space toggle  ·  Tab next question  ·  Enter confirm  ·  Esc cancel")

	var sections []string
	sections = append(sections, title, hint,
		lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", w-4)))

	for i, q := range d.Questions {
		focused := i == d.focusedQ

		// Chip label
		chipStyle := lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#E8F0FE", Dark: "#1E3A5F"}).
			Foreground(accentColor).
			Padding(0, 1).
			Bold(true)
		chip := chipStyle.Render(q.Header)

		// Question text
		qStyle := lipgloss.NewStyle().Foreground(headerColor)
		if focused {
			qStyle = qStyle.Bold(true)
		}
		questionLine := chip + "  " + qStyle.Render(q.Question)
		sections = append(sections, questionLine)

		switch q.Type {
		case agent_domain.QuestionTypeChoice:
			for j, opt := range q.Options {
				isSelected := d.cursors[i] == j
				isChecked := d.multiSelected[i][j]

				var prefix string
				if q.MultiSelect {
					if isChecked {
						prefix = lipgloss.NewStyle().Foreground(checkColor).Render("[✓]")
					} else {
						prefix = lipgloss.NewStyle().Foreground(mutedColor).Render("[ ]")
					}
				} else {
					if isSelected {
						prefix = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("▸")
					} else {
						prefix = "  "
					}
				}

				label := opt.Label
				desc := ""
				if opt.Description != "" {
					desc = "  " + lipgloss.NewStyle().Foreground(mutedColor).Render(opt.Description)
				}

				var row string
				if isSelected {
					labelStyled := lipgloss.NewStyle().Bold(true).Foreground(headerColor).Render(label)
					content := fmt.Sprintf("  %s %s%s", prefix, labelStyled, desc)
					row = lipgloss.NewStyle().Background(selectedBg).Width(w - 4).Render(content)
				} else {
					content := fmt.Sprintf("  %s %s%s", prefix,
						lipgloss.NewStyle().Foreground(headerColor).Render(label), desc)
					row = lipgloss.NewStyle().Width(w - 4).Render(content)
				}
				sections = append(sections, row)
			}

		case agent_domain.QuestionTypeYesNo:
			yesno := []struct{ label string }{{"Yes"}, {"No"}}
			for j, yn := range yesno {
				isSelected := d.cursors[i] == j
				prefix := "  "
				if isSelected {
					prefix = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("▸")
				}
				var row string
				if isSelected {
					row = lipgloss.NewStyle().Background(selectedBg).Width(w - 4).
						Render(fmt.Sprintf("  %s %s", prefix,
							lipgloss.NewStyle().Bold(true).Foreground(headerColor).Render(yn.label)))
				} else {
					row = lipgloss.NewStyle().Width(w - 4).
						Render(fmt.Sprintf("  %s %s", prefix,
							lipgloss.NewStyle().Foreground(headerColor).Render(yn.label)))
				}
				sections = append(sections, row)
			}

		case agent_domain.QuestionTypeText:
			ph := q.Placeholder
			if ph == "" {
				ph = "Type your answer…"
			}
			val := d.textValues[i]
			displayed := val
			if displayed == "" {
				displayed = lipgloss.NewStyle().Foreground(mutedColor).Render(ph)
			}
			cursor := ""
			if focused {
				cursor = lipgloss.NewStyle().Foreground(accentColor).Render("▌")
			}
			inputLine := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(func() lipgloss.AdaptiveColor {
					if focused {
						return accentColor
					}
					return borderColor
				}()).
				Width(w-8).
				Padding(0, 1).
				Render(displayed + cursor)
			sections = append(sections, "  "+inputLine)
		}

		sections = append(sections, "") // spacing between questions
	}

	// Footer
	sections = append(sections,
		lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", w-4)),
		lipgloss.NewStyle().Foreground(mutedColor).Render("Press Enter to confirm · Esc to cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(w).
		Padding(1, 2).
		Render(content)
}

// CompactView renders a small inline prompt for single yes/no questions.
func (d *AskUserDialog) CompactView(w int) string {
	accentColor := lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#5B9BD5"}
	borderColor := lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
	textColor := lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	mutedColor := lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}

	q := d.Questions[0]

	// Clean up newlines to make it a horizontal bar
	questionStr := strings.ReplaceAll(strings.TrimSpace(q.Question), "\n\n", "  ·  ")
	questionStr = strings.ReplaceAll(questionStr, "\n", " ")
	for strings.Contains(questionStr, "  ") && !strings.Contains(questionStr, "  ·  ") {
		questionStr = strings.ReplaceAll(questionStr, "  ", " ")
	}

	chipStyle := lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "#E8F0FE", Dark: "#1E3A5F"}).
		Foreground(accentColor).
		Padding(0, 1).
		Bold(true)

	header := chipStyle.Render(q.Header)
	questionText := lipgloss.NewStyle().Foreground(textColor).Render(questionStr)

	plainBtn := lipgloss.NewStyle().Padding(0, 1)
	activeBtn := lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "#EEF2FF", Dark: "#2A2D3E"}).
		Foreground(accentColor).
		Bold(true).
		Padding(0, 1)

	yesLabel, noLabel := "Yes", "No"
	yesStyle, noStyle := plainBtn, plainBtn
	if d.cursors[0] == 0 {
		yesLabel = "▸ Yes"
		yesStyle = activeBtn
	} else {
		noLabel = "▸ No"
		noStyle = activeBtn
	}

	btns := lipgloss.JoinHorizontal(lipgloss.Center,
		yesStyle.Render(yesLabel), "  ", noStyle.Render(noLabel),
	)

	hint := lipgloss.NewStyle().Foreground(mutedColor).Render("(Enter: confirm, Esc: cancel)")

	// Layout: Top line is header + question, Bottom line is buttons + hint
	topLine := header + "  " + questionText
	
	// Create an elegant left padding for the second row so buttons align nicely.
	bottomLine := lipgloss.JoinHorizontal(lipgloss.Center, "    ", btns, "    ", hint)

	content := lipgloss.JoinVertical(lipgloss.Left, topLine, bottomLine)

	// A thin, wide rectangle shrink-wrapping dynamically to the content width.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(content)
}
