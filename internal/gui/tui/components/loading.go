package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

var (
	loadingAccent = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}
	loadingText   = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
)

// RenderLoading renders a one-line loading indicator at the given width.
// The spinner model's View() is passed in to avoid import cycles.
func RenderLoading(sp spinner.Model, width int, isLoading bool) string {
	if !isLoading {
		// Render an empty line to keep layout stable.
		return lipgloss.NewStyle().Width(width).Height(1).Render("")
	}

	label := lipgloss.NewStyle().Foreground(loadingText).Render(" Thinking…")
	line := sp.View() + label

	return lipgloss.NewStyle().
		Width(width).
		Height(1).
		PaddingLeft(1).
		Render(line)
}
