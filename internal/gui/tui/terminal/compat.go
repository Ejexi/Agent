package terminal

import (
	"os"
	"strings"

	"github.com/muesli/termenv"
)

// TerminalCapabilities holds detected terminal features and fallback hints.
type TerminalCapabilities struct {
	ColorProfile termenv.Profile // Ascii | ANSI | ANSI256 | TrueColor
	Unicode      bool           // Can render Unicode box-drawing chars
	MouseSupport bool           // Safe to enable mouse capture
	TrueColor    bool           // 16M color support
	IsLegacy     bool           // CMD, XTerm, URxvt, LXTerminal
	IsTmux       bool           // Running inside tmux/screen
	Name         string         // Detected terminal name
}

// DetectTerminal inspects environment variables and the termenv library to
// build a capability profile that the rest of the TUI can query.
func DetectTerminal() TerminalCapabilities {
	caps := TerminalCapabilities{}

	// ── Detect terminal name ────────────────────────────────────────
	termProgram := os.Getenv("TERM_PROGRAM")
	term := os.Getenv("TERM")

	switch {
	case os.Getenv("WT_SESSION") != "":
		caps.Name = "Windows Terminal"
	case os.Getenv("KITTY_WINDOW_ID") != "":
		caps.Name = "Kitty"
	case strings.EqualFold(termProgram, "iTerm.app"):
		caps.Name = "iTerm2"
	case strings.EqualFold(termProgram, "WarpTerminal"):
		caps.Name = "Warp"
	case strings.EqualFold(termProgram, "Apple_Terminal"):
		caps.Name = "Terminal.app"
	case strings.EqualFold(termProgram, "Hyper"):
		caps.Name = "Hyper"
	case strings.Contains(strings.ToLower(termProgram), "alacritty"):
		caps.Name = "Alacritty"
	case strings.Contains(strings.ToLower(term), "xterm"):
		caps.Name = "XTerm"
	case strings.Contains(strings.ToLower(term), "rxvt"):
		caps.Name = "URxvt"
	case os.Getenv("ConEmuPID") != "":
		caps.Name = "ConEmu"
	default:
		if term != "" {
			caps.Name = term
		} else {
			caps.Name = "unknown"
		}
	}

	// ── Color profile ───────────────────────────────────────────────
	caps.ColorProfile = termenv.ColorProfile()
	caps.TrueColor = caps.ColorProfile == termenv.TrueColor

	// ── Legacy detection ────────────────────────────────────────────
	legacyNames := []string{"cmd", "xterm", "urxvt", "lxterminal", "mintty", "linux"}
	nameLower := strings.ToLower(caps.Name)
	for _, l := range legacyNames {
		if strings.Contains(nameLower, l) {
			caps.IsLegacy = true
			break
		}
	}

	// ── Unicode support ─────────────────────────────────────────────
	// Legacy terminals often lack good Unicode box-drawing support.
	caps.Unicode = !caps.IsLegacy

	// ── tmux / screen ───────────────────────────────────────────────
	caps.IsTmux = os.Getenv("TMUX") != "" || strings.HasPrefix(term, "screen")

	// ── Mouse support ───────────────────────────────────────────────
	// Mouse is generally safe on modern terminals but must NOT be on by
	// default (breaks text selection in some terminals).
	caps.MouseSupport = !caps.IsLegacy && !caps.IsTmux

	return caps
}
