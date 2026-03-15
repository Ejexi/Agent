package components

import (
	"github.com/charmbracelet/lipgloss"
)

// ToastLevel mirrors the model-level enum (avoids import cycle).
const (
	ToastLevelInfo    = 0
	ToastLevelSuccess = 1
	ToastLevelWarning = 2
	ToastLevelError   = 3
)

var (
	toastColors = map[int]lipgloss.AdaptiveColor{
		ToastLevelInfo:    {Light: "#1565C0", Dark: "#64B5F6"},
		ToastLevelSuccess: {Light: "#2E7D32", Dark: "#34D399"},
		ToastLevelWarning: {Light: "#F57C00", Dark: "#FFB74D"},
		ToastLevelError:   {Light: "#D32F2F", Dark: "#FF6B6B"},
	}
	toastText = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
)

// RenderToast renders a toast notification positioned at the top-right.
func RenderToast(message string, level int, termW, termH int) string {
	accent, ok := toastColors[level]
	if !ok {
		accent = toastColors[ToastLevelInfo]
	}

	toastW := 40
	if toastW > termW-4 {
		toastW = termW - 4
	}

	toast := lipgloss.NewStyle().
		Foreground(toastText).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		BorderLeft(true).
		BorderRight(true).
		BorderTop(true).
		BorderBottom(true).
		Padding(0, 1).
		Width(toastW).
		Render(message)

	return toast
}
