package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	popupBorder = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}
	popupTitle  = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}
	popupText   = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	popupMuted  = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
)

// RenderShortcutsPopup renders the keyboard shortcuts popup.
func RenderShortcutsPopup(termW, termH int, isLegacy bool) string {
	// ── Colors & Styles ─────────────────────────────────────────────
	bgColor := lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1A1A1A"}
	
	baseStyle := lipgloss.NewStyle().Background(bgColor)
	
	titleStyle := baseStyle.Copy().
		Foreground(popupTitle).
		Bold(true)

	keyStyle := baseStyle.Copy().
		Foreground(popupTitle).
		Bold(true)

	descStyle := baseStyle.Copy().
		Foreground(popupText)

	mutedStyle := baseStyle.Copy().
		Foreground(popupMuted).
		Italic(true)

	sepColor := lipgloss.AdaptiveColor{Light: "#DDDDDD", Dark: "#333333"}
	sepStyle := baseStyle.Copy().Foreground(sepColor)

	popupW := 60
	if popupW > termW-4 {
		popupW = termW - 4
	}

	// ── Content Building ────────────────────────────────────────────
	title := titleStyle.Render("KEYBOARD SHORTCUTS")
	sep := sepStyle.Render(strings.Repeat("─", popupW-4))

	shortcuts := []struct{ key, desc string }{
		{"Ctrl+C", "Quit Application"},
		{"Ctrl+K", "Toggle this help"},
		{"Tab", "Toggle side panel"},
		{"Esc", "Close current view"},
		{"Enter", "Send message"},
		{"Alt+Enter", "Add new line (\\n)"},
		{"/", "Open command menu"},
		{"!", "Terminal mode prefix"},
		{"@", "File search prefix"},
		{"↑ / ↓", "Scroll chat history"},
		{"Ctrl+F", "Toggle terminal focus"},
	}

	var rows []string
	for _, s := range shortcuts {
		// Use fixed-width rendering for the key column via fmt.Sprintf to avoid style-width issues
		key := fmt.Sprintf("%-16s", s.key)
		rows = append(rows, keyStyle.Render(key)+" "+descStyle.Render(s.desc))
	}

	hint := mutedStyle.Render("Press Esc or Ctrl+K to close")

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		sep,
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		"",
		sep,
		hint,
	)

	// ── Container ───────────────────────────────────────────────────
	border := lipgloss.RoundedBorder()
	if isLegacy {
		border = lipgloss.NormalBorder()
	}

	// Height is content lines + padding
	contentLines := strings.Split(content, "\n")
	popupH := len(contentLines) + 2

	modalStyle := lipgloss.NewStyle().
		BorderStyle(border).
		BorderForeground(popupBorder).
		Background(bgColor).
		Padding(1, 2).
		Width(popupW).
		Height(popupH)

	// Place the content in the center of the modal box
	return modalStyle.Render(content)
}
