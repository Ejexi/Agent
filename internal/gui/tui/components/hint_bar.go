package components

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	hintKey  = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}
	hintDesc = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	hintSep  = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
)

// RenderHintBar renders a single-line keyboard shortcut bar at the given width.
func RenderHintBar(width int) string {
	keyStyle := lipgloss.NewStyle().Foreground(hintKey).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(hintDesc)
	sepStyle := lipgloss.NewStyle().Foreground(hintSep)

	sep := sepStyle.Render(" │ ")

	items := []string{
		keyStyle.Render("Ctrl+C") + descStyle.Render(" quit"),
		keyStyle.Render("Enter") + descStyle.Render(" send"),
		keyStyle.Render("Tab") + descStyle.Render(" panel"),
		keyStyle.Render("/") + descStyle.Render(" cmds"),
		keyStyle.Render("Ctrl+K") + descStyle.Render(" keys"),
	}

	bar := ""
	for i, item := range items {
		if i > 0 {
			bar += sep
		}
		bar += item
	}

	return lipgloss.NewStyle().
		Width(width).
		PaddingLeft(1).
		Render(bar)
}
