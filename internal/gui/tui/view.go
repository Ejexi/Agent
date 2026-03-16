package tui

import (
	"strings"

	"github.com/SecDuckOps/agent/internal/gui/tui/components"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "\n  Initializing DuckOps…"
	}

	// ── Shell Mode Handling ─────────────────────────────────────────
	// (Handled inside content stacking below)

	// ── Protect against very small terminals ────────────────────────
	if m.width < 20 || m.height < 8 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(Theme.Warning).Render("Terminal too small"))
	}

	// ── Calculate main content width ────────────────────────────────
	mainW := m.width
	if m.showSidePanel {
		mainW = m.width - components.SidePanelWidth
		if mainW < 20 {
			mainW = 20
		}
	}

	// ── Fixed-height components ─────────────────────────────────────
	inputH := components.InputHeight(m.textarea)
	hintH := 1
	loadingH := 1
	
	cwd := ""
	if m.engine != nil {
		cwd = m.engine.GetCwd()
	}
	
	header := components.HeaderView(m.width, "0.2.0", "admin")
	headerH := lipgloss.Height(header)

	// ── Overlay/Stack Components ────────────────────────────────────
	var menu string
	var menuH int
	if m.showMenu {
		items := components.GetFilteredMenuItems(m.textarea.Value())
		menu = components.Menu(items, m.menuSelection)
		menuH = lipgloss.Height(menu)
	}

	var drawer string
	var drawerH int
	if m.mode == ShellDiscoveryMode && len(m.discoveryResults) > 0 {
		items := make([]interface{}, len(m.discoveryResults))
		for i, res := range m.discoveryResults {
			items[i] = map[string]string{
				"name": res.Name,
				"info": res.Path,
			}
		}
		drawer = components.CommandDrawer(items, m.discoveryIndex, mainW, "System Commands")
		drawerH = lipgloss.Height(drawer)
	} else if m.mode == FileDiscoveryMode && len(m.discoveryResults) > 0 {
		items := make([]interface{}, len(m.discoveryResults))
		for i, res := range m.discoveryResults {
			items[i] = map[string]string{
				"name": res.Name,
				"info": res.Path,
			}
		}
		drawer = components.CommandDrawer(items, m.discoveryIndex, mainW, "Files & Directories")
		drawerH = lipgloss.Height(drawer)
	}

	// ── Messages = everything remaining ─────────────────────────────
	msgsH := m.height - inputH - hintH - loadingH - headerH - menuH - drawerH
	if m.activePopup != PopupNone {
		// When popup is active, we don't show drawer/menu/loading, so give it that space.
		msgsH = m.height - inputH - hintH - headerH
	}
	if msgsH < 1 {
		msgsH = 1
	}

	// ── Build chat messages for the component ───────────────────────
	chatMsgs := make([]components.ChatMessage, len(m.messages))
	for i, msg := range m.messages {
		chatMsgs[i] = components.ChatMessage{
			Type:         int(msg.Type),
			Content:      msg.Content,
			Sender:       msg.Sender,
			Timestamp:    msg.Timestamp,
			TableHeaders: msg.TableHeaders,
			TableData:    msg.TableData,
		}
	}

	// ── Render Area components ──────────────────────────────────────
	var msgs string
	var input string
	var loading string
	var isShell = m.mode == ShellDiscoveryMode || m.mode == ShellExecutionMode

	if m.activePopup != PopupNone {
		// When a popup is active, we render an empty dimmed area for messages
		// to create a "Z-index / Blur" focus on the popup.
		msgs = lipgloss.Place(mainW, msgsH, lipgloss.Center, lipgloss.Center,
			components.RenderShortcutsPopup(mainW, msgsH, m.caps.IsLegacy))
		input = components.RenderInput(m.textarea, mainW, m.caps.IsLegacy, isShell)
		// Dim the input while popup is active
		input = dim(input)
	} else {
		msgs = components.RenderMessageArea(chatMsgs, mainW, msgsH, m.scroll, m.logo, m.dynamicSuggestions)
		loading = components.RenderLoading(m.spinner, mainW, m.loading)
		input = components.RenderInput(m.textarea, mainW, m.caps.IsLegacy, isShell)
	}

	hint := components.RenderHintBar(mainW, cwd)

	// 1. Join Chat Area components
	var chatArea string
	if m.activePopup != PopupNone {
		// Dim the hint bar as well
		dimmedHint := dim(hint)
		// We stack: [Pop-up in Messages Area] + [Dimmed Input] + [Dimmed Hint]
		chatArea = lipgloss.JoinVertical(lipgloss.Left, msgs, input, dimmedHint)
	} else {
		chatElements := []string{msgs, loading}
		if drawer != "" {
			chatElements = append(chatElements, drawer)
		}
		if menu != "" {
			chatElements = append(chatElements, menu)
		}
		chatElements = append(chatElements, input, hint)
		chatArea = lipgloss.JoinVertical(lipgloss.Left, chatElements...)
	}

	// 2. Join Horizontal with Side Panel if open
	var body string
	if m.showSidePanel {
		panelH := m.height - headerH
		panel := components.RenderSidePanel(panelH, m.activeModel, m.totalUsage.PromptTokens, m.totalUsage.CompletionTokens)
		if m.activePopup != PopupNone {
			panel = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#333333"}).Render(panel)
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, chatArea, panel)
	} else {
		body = chatArea
	}

	// 3. Add Header on top
	if m.activePopup != PopupNone {
		header = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#333333"}).Render(header)
	}
	main := lipgloss.JoinVertical(lipgloss.Left, header, body)

	// ── Final View ──────────────────────────────────────────────────
	result := lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, main)

	// ── Toasts (Non-blocking overlays) ──────────────────────────────
	if m.toast != nil {
		toast := components.RenderToast(m.toast.Message, int(m.toast.Level), m.width, m.height)
		tw, th := lipgloss.Width(toast), lipgloss.Height(toast)
		result = overlayAt(result, toast, m.width-tw-1, 1, m.width, m.height, tw, th)
	}

	return result
}

// overlayAt places `overlay` on top of `base` at a given (x, y) character
// position.  This is a simple string-based overlay (splits by newline).
func overlayAt(base, overlay string, x, y, baseW, baseH, overlayW, overlayH int) string {
	baseLines := splitLines(base, baseH)
	overlayLines := splitLines(overlay, overlayH)

	for i, ol := range overlayLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}

		// Work with runes to handle multi-byte characters correctly
		bl := []rune(baseLines[row])

		// If the base line is shorter than what we need to overlay, pad it
		neededLen := x + len([]rune(ol))
		if len(bl) < neededLen {
			padding := make([]rune, neededLen-len(bl))
			for k := range padding {
				padding[k] = ' '
			}
			bl = append(bl, padding...)
		}

		olRunes := []rune(ol)
		for j, r := range olRunes {
			col := x + j
			if col >= 0 && col < len(bl) {
				bl[col] = r
			}
		}
		baseLines[row] = string(bl)
	}

	return strings.Join(baseLines, "\n")
}

func splitLines(s string, expectedH int) []string {
	lines := strings.Split(s, "\n")
	if len(lines) > expectedH {
		return lines[:expectedH]
	}
	// Pad with empty lines if shoter
	for len(lines) < expectedH {
		lines = append(lines, "")
	}
	return lines
}

func dim(s string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#222222"}).
		Render(s)
}
