package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	hintKey  = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}
	hintDesc = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	hintSep  = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
)

// RenderHintBar renders a single-line keyboard shortcut bar at the given width.
func RenderHintBar(width int, cwd string) string {
	keyStyle := lipgloss.NewStyle().Foreground(hintKey).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(hintDesc)
	sepStyle := lipgloss.NewStyle().Foreground(hintSep)

	sep := sepStyle.Render(" │ ")

	items := []string{
		keyStyle.Render("Ctrl+C") + descStyle.Render(" quit"),
		keyStyle.Render("Enter") + descStyle.Render(" send"),
		keyStyle.Render("Ctrl+B") + descStyle.Render(" panel"),
		keyStyle.Render("/") + descStyle.Render(" cmds"),
		keyStyle.Render("Ctrl+K") + descStyle.Render(" keys"),
	}

	rightBar := ""
	for i, item := range items {
		if i > 0 {
			rightBar += sep
		}
		rightBar += item
	}

	leftBar := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#2A2A2A"}). // Match chip background style
		Padding(0, 2).                                                         // Wider padding to match User's request `  D:\...  `
		MarginLeft(1).
		Render(cwd)

	// Calculate space between left (cwd) and right (keys)
	space := width - lipgloss.Width(leftBar) - lipgloss.Width(rightBar) - 1 // 1 for right padding
	if space < 0 {
		space = 0
	}

	return lipgloss.JoinHorizontal(lipgloss.Bottom,
		leftBar,
		strings.Repeat(" ", space),
		rightBar,
	)
}
