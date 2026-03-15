package tui

import (
	"github.com/SecDuckOps/agent/internal/gui/tui/terminal"

	"github.com/charmbracelet/lipgloss"
)

// Theme holds the adaptive color palette for the TUI. Every color uses
// AdaptiveColor so lipgloss auto-selects the value matching the terminal's
// background luminance (light vs dark).
var Theme = struct {
	Text    lipgloss.AdaptiveColor
	Muted   lipgloss.AdaptiveColor
	Accent  lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor
	Error   lipgloss.AdaptiveColor
	Success lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Border  lipgloss.AdaptiveColor
	Subtle  lipgloss.AdaptiveColor
}{
	Text:    lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"},
	Muted:   lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"},
	Accent:  lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"},
	Secondary: lipgloss.AdaptiveColor{Light: "#00BCD4", Dark: "#88C0D0"},
	Error:   lipgloss.AdaptiveColor{Light: "#D32F2F", Dark: "#FF6B6B"},
	Success: lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#34D399"},
	Warning: lipgloss.AdaptiveColor{Light: "#F57C00", Dark: "#FFB74D"},
	Border:  lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"},
	Subtle:  lipgloss.AdaptiveColor{Light: "#EEEEEE", Dark: "#1a1a1a"},
}

// BorderStyle returns a RoundedBorder for modern terminals and a NormalBorder
// (ASCII +--+) for legacy ones.
func BorderStyle(caps terminal.TerminalCapabilities) lipgloss.Border {
	if caps.IsLegacy {
		return lipgloss.NormalBorder()
	}
	return lipgloss.RoundedBorder()
}

// SpinnerChars returns braille dots for Unicode-capable terminals and an ASCII
// sequence for legacy ones.
func SpinnerChars(caps terminal.TerminalCapabilities) []string {
	if caps.Unicode {
		return []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	}
	return []string{"|", "/", "-", "\\"}
}

// ── Convenience style builders ─────────────────────────────────────

// BaseText returns a style with only the adaptive text foreground set.
// Background is intentionally NOT set so the terminal background shows through.
func BaseText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Theme.Text)
}

// MutedText returns a dimmed style for secondary information.
func MutedText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Theme.Muted)
}

// AccentText returns a style using the accent colour.
func AccentText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Theme.Accent)
}

// ErrorText returns a style using the error colour.
func ErrorText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Theme.Error)
}

// SuccessText returns a style using the success colour.
func SuccessText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Theme.Success)
}
