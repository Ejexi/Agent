package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	popupBorder = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}
	popupTitle  = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}
	popupText   = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	popupMuted  = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	popupBg     = lipgloss.AdaptiveColor{Light: "", Dark: ""}
)

// ── Shared helpers ───────────────────────────────────────────────────

// popupDims calculates outer and inner widths clamped to terminal width.
func popupDims(termW, preferredW int) (popupW, innerW int) {
	popupW = preferredW
	if popupW > termW-4 {
		popupW = termW - 4
	}
	innerW = popupW - 6
	if innerW < 10 {
		innerW = 10
	}
	return
}

// line renders a string at a fixed width with given alignment,
// manually padding with styled spaces to prevent ANSI background bleed.
func line(s string, w int, align lipgloss.Position) string {
	visibleW := lipgloss.Width(s)
	if visibleW >= w {
		return s
	}

	padTotal := w - visibleW
	var padLeft, padRight int
	// Convert lipgloss.Position (float64) to meaning: 0=Left, 0.5=Center, 1=Right
	if align == lipgloss.Left {
		padRight = padTotal
	} else if align == lipgloss.Right {
		padLeft = padTotal
	} else {
		padLeft = padTotal / 2
		padRight = padTotal - padLeft
	}

	bgStyle := lipgloss.NewStyle().Background(popupBg)
	res := ""
	if padLeft > 0 {
		res += bgStyle.Render(strings.Repeat(" ", padLeft))
	}
	res += s
	if padRight > 0 {
		res += bgStyle.Render(strings.Repeat(" ", padRight))
	}
	return res
}

// sep renders a horizontal separator at a fixed width.
func sep(w int) string {
	return lipgloss.NewStyle().
		Width(w).
		Background(popupBg).
		Foreground(lipgloss.AdaptiveColor{Light: "#DDDDDD", Dark: "#333333"}).
		Render(strings.Repeat("─", w))
}

// popupModal wraps content in a bordered box with background.
func popupModal(content string, popupW int, isLegacy bool) string {
	border := lipgloss.RoundedBorder()
	if isLegacy {
		border = lipgloss.NormalBorder()
	}
	return lipgloss.NewStyle().
		BorderStyle(border).
		BorderForeground(popupBorder).
		Background(popupBg).
		Padding(1, 2).
		Width(popupW).
		Render(content)
}

// ── Shortcuts popup ──────────────────────────────────────────────────

func RenderShortcutsPopup(termW, termH int, isLegacy bool) string {
	popupW, innerW := popupDims(termW, 60)

	titleStyle := lipgloss.NewStyle().Background(popupBg).Foreground(popupTitle).Bold(true)
	keyStyle := lipgloss.NewStyle().Background(popupBg).Foreground(popupTitle).Bold(true)
	descStyle := lipgloss.NewStyle().Background(popupBg).Foreground(popupText)
	mutedStyle := lipgloss.NewStyle().Background(popupBg).Foreground(popupMuted).Italic(true)

	shortcuts := []struct{ key, desc string }{
		{"Ctrl+C", "Quit Application"},
		{"Ctrl+K", "Toggle this help"},
		{"Ctrl+B", "Toggle side panel"},
		{"Esc", "Close current view"},
		{"Enter", "Send message"},
		{"/", "Open command menu"},
		{"!", "Terminal mode prefix"},
		{"@", "File search prefix"},
		{"↑ / ↓", "Scroll chat history"},
		{"Ctrl+F", "Toggle terminal focus"},
	}

	var lines []string
	lines = append(lines,
		line(titleStyle.Render("KEYBOARD SHORTCUTS"), innerW, lipgloss.Center),
		sep(innerW),
		"",
	)
	for _, sc := range shortcuts {
		row := keyStyle.Render(fmt.Sprintf("%-12s", sc.key)) + lipgloss.NewStyle().Background(popupBg).Render("  ") + descStyle.Render(sc.desc)
		lines = append(lines, line(row, innerW, lipgloss.Left))
	}
	lines = append(lines,
		"",
		sep(innerW),
		line(mutedStyle.Render("Press Esc or Ctrl+K to close"), innerW, lipgloss.Center),
	)

	return popupModal(strings.Join(lines, "\n"), popupW, isLegacy)
}

// ── Exit popup ───────────────────────────────────────────────────────

func RenderExitPopup(termW, termH int, isLegacy bool, selectedYes bool) string {
	const question = "Are you sure you want to exit DuckOps?"
	popupW, innerW := popupDims(termW, lipgloss.Width(question)+8)

	accentColor := lipgloss.AdaptiveColor{Light: "#E53935", Dark: "#EF5350"}
	titleStyle := lipgloss.NewStyle().Background(popupBg).Foreground(accentColor).Bold(true)
	textStyle := lipgloss.NewStyle().Background(popupBg).Foreground(popupText)
	mutedStyle := lipgloss.NewStyle().Background(popupBg).Foreground(popupMuted).Italic(true)

	plainBtn := lipgloss.NewStyle().Background(popupBg).Padding(0, 2)
	activeBtn := lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "#FFEBEE", Dark: "#3A1A1A"}).
		Foreground(accentColor).
		Bold(true).
		Padding(0, 2)

	yesLabel, noLabel := "  Yes", "  No "
	yesStyle, noStyle := plainBtn, plainBtn
	if selectedYes {
		yesLabel = "▸ Yes"
		yesStyle = activeBtn
	} else {
		noLabel = "▸ No "
		noStyle = activeBtn
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center,
		yesStyle.Render(yesLabel), lipgloss.NewStyle().Background(popupBg).Render("  "), noStyle.Render(noLabel),
	)

	lines := []string{
		line(titleStyle.Render("🦆 EXIT DUCKOPS"), innerW, lipgloss.Center),
		sep(innerW),
		"",
		line(textStyle.Render(question), innerW, lipgloss.Center),
		"",
		line(buttons, innerW, lipgloss.Center),
		"",
		sep(innerW),
		line(mutedStyle.Render("(Enter to confirm, Esc to cancel)"), innerW, lipgloss.Center),
	}

	return popupModal(strings.Join(lines, "\n"), popupW, isLegacy)
}
