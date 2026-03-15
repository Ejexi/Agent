package components

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
)

var (
	inputBorderColor = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
	inputAccent      = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}
)

// RenderInput renders the textarea inside a bordered box at the given width.
// If isShell is true, it uses a Cyan glow effect.
func RenderInput(ta textarea.Model, width int, isLegacy bool, isShell bool) string {
	border := lipgloss.RoundedBorder()
	if isLegacy {
		border = lipgloss.NormalBorder()
	}

	accent := inputAccent
	if isShell {
		accent = lipgloss.AdaptiveColor{Light: "#00FFFF", Dark: "#00FFFF"} // Cyan glow
	}

	style := lipgloss.NewStyle().
		BorderStyle(border).
		BorderForeground(accent).
		Width(width - 2). // account for border width
		PaddingLeft(1)

	return style.Render(ta.View())
}

// InputHeight returns the total outer height of the input component
// for layout calculations (textarea height + 2 for border).
func InputHeight(ta textarea.Model) int {
	return ta.Height() + 2
}
