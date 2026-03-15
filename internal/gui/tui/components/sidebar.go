package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SidePanelWidth is the fixed width of the side panel (including border).
const SidePanelWidth = 32

var (
	sidePanelBorder = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
	sidePanelTitle  = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}
	sidePanelText   = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	sidePanelMuted  = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
)

// RenderSidePanel renders the side panel at fixed width and the given height.
func RenderSidePanel(height int, modelName string, promptTokens, completionTokens int) string {
	innerW := SidePanelWidth - 3 // border (1) + padding (2)

	// Format tokens
	formatTokens := func(n int) string {
		if n == 0 {
			return "-"
		}
		if n >= 1000 {
			return fmt.Sprintf("%.1fk", float64(n)/1000.0)
		}
		return fmt.Sprintf("%d", n)
	}

	title := lipgloss.NewStyle().
		Foreground(sidePanelTitle).
		Bold(true).
		Width(innerW).
		Render("Agent Info")

	usageTitle := lipgloss.NewStyle().
		Foreground(sidePanelTitle).
		Bold(true).
		Width(innerW).
		PaddingTop(1).
		Render("Token Usage")

	separator := lipgloss.NewStyle().
		Foreground(sidePanelBorder).
		Width(innerW).
		Render(strings.Repeat("─", innerW))

	items := []string{
		renderSideItem("Model", modelName, innerW),
		renderSideItem("Mode", "TUI", innerW),
	}

	usageItems := []string{
		renderSideItem("Prompt", formatTokens(promptTokens), innerW),
		renderSideItem("Reply", formatTokens(completionTokens), innerW),
		renderSideItem("Total", formatTokens(promptTokens+completionTokens), innerW),
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		separator,
		strings.Join(items, "\n"),
		usageTitle,
		separator,
		strings.Join(usageItems, "\n"),
	)

	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderLeft(true).
		BorderRight(false).
		BorderTop(false).
		BorderBottom(false).
		BorderForeground(sidePanelBorder).
		Width(SidePanelWidth).
		Height(height).
		PaddingLeft(1)

	return style.Render(content)
}

func renderSideItem(label, value string, width int) string {
	labelStyle := lipgloss.NewStyle().Foreground(sidePanelMuted)
	valueStyle := lipgloss.NewStyle().Foreground(sidePanelText)

	return lipgloss.NewStyle().Width(width).Render(
		labelStyle.Render(label+": ") + valueStyle.Render(value),
	)
}
