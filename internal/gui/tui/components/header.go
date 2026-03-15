package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerAccent = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}
	headerText   = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	headerMuted  = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	headerBorder = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
)

// HeaderView renders the top header bar at the given width.
func HeaderView(width int, version string, user string) string {
	// ── Bottom bar: version  | user ─────────────────────────
	vText := lipgloss.NewStyle().
		Foreground(headerMuted).
		PaddingLeft(1).
		Render("DuckOps " + version)

	uText := lipgloss.NewStyle().
		Foreground(headerMuted).
		PaddingRight(1).
		Render("USER: " + strings.ToUpper(user))

	rightSide := lipgloss.JoinHorizontal(lipgloss.Bottom, " ", uText)

	calcSpace := width - lipgloss.Width(vText) - lipgloss.Width(rightSide)
	if calcSpace < 0 {
		calcSpace = 0
	}

	bottomBar := lipgloss.JoinHorizontal(lipgloss.Bottom,
		vText,
		strings.Repeat(" ", calcSpace),
		rightSide,
	)

	separator := lipgloss.NewStyle().
		Foreground(headerBorder).
		Render(strings.Repeat("─", width))

	return lipgloss.JoinVertical(lipgloss.Left, bottomBar, separator)
}
