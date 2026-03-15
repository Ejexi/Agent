package tui

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/subagent"
	"github.com/SecDuckOps/agent/internal/engine"
	"github.com/SecDuckOps/agent/internal/gui/tui/components"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/shlex"
)

var ansiRegex = regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// ── Custom messages ─────────────────────────────────────────────────

type agentResponseMsg struct {
	content string
	usage   shared_domain.TokenUsage
	model   string
	err     error
}

type toastDismissMsg struct{}

// ── Agent command ───────────────────────────────────────────────────

func (m model) runAgent(userInput string) tea.Cmd {
	return func() tea.Msg {
		ch, err := m.engine.StreamChat(context.Background(), userInput)
		if err != nil {
			return agentResponseMsg{err: err}
		}
		return agentStreamMsg{ch: ch}
	}
}

type agentStreamMsg struct {
	ch <-chan any
}

func waitForAgentEvent(ch <-chan any) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return nil // Stream closed
		}
		return evt
	}
}

// ── Update ──────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Delegate to shell first if it needs to handle PTY output or exit
	switch msg := msg.(type) {
	case ShellOutputMsg:
		if len(m.messages) > 0 {
			last := &m.messages[len(m.messages)-1]
			if last.Type == AgentMsg && last.Sender == "DuckOps" {
				// Strip code block markers if present to append inside
				content := last.Content
				if strings.HasSuffix(content, "```") {
					content = strings.TrimSuffix(content, "```")
				}
				if !strings.Contains(content, "```shell") {
					content += "```shell\n"
				}
				content += stripANSI(string(msg))
				last.Content = content + "```"
			}
		}
		var cmd tea.Cmd
		m.shell, cmd = m.shell.Update(msg)
		return m, cmd
	case ShellExitMsg:
		var cmd tea.Cmd
		m.shell, cmd = m.shell.Update(msg)
		m.loading = false
		m.isProcessing = false
		return m, cmd
	}

	switch msg := msg.(type) {
	case agentStreamMsg:
		m.lastStreamCh = msg.ch
		return m, waitForAgentEvent(msg.ch)

	case subagent.SubagentEvent:
		m.messages = append(m.messages, Message{
			Type:      LearningMsg,
			Content:   msg.Message,
			Sender:    "DuckOps",
			Timestamp: msg.Timestamp,
		})
		m.scroll = 0
		m.stayAtBottom = true
		return m, waitForAgentEvent(m.lastStreamCh)

	// ── Agent response (Final Result) ────────────────────────────────
	case engine.ChatResult:
		m.isProcessing = false
		m.loading = false
		m.messages = append(m.messages, Message{
			Type:      AgentMsg,
			Content:   msg.Content,
			Sender:    "DuckOps",
			Timestamp: time.Now(),
		})
		
		// Update telemetry
		m.totalUsage.PromptTokens += msg.Usage.PromptTokens
		m.totalUsage.CompletionTokens += msg.Usage.CompletionTokens
		m.totalUsage.TotalTokens += msg.Usage.TotalTokens
		if msg.Model != "" {
			m.activeModel = msg.Model
		}
		m.scroll = 0
		m.stayAtBottom = true
		return m, nil

	case agentResponseMsg:
		m.isProcessing = false
		m.loading = false
		if msg.err != nil {
			m.messages = append(m.messages, Message{
				Type:      ErrorMsg,
				Content:   "Error: " + msg.err.Error(),
				Sender:    "DuckOps",
				Timestamp: time.Now(),
			})
		}
		return m, nil

	case error: // Catch-all for engine errors in stream
		m.isProcessing = false
		m.loading = false
		m.messages = append(m.messages, Message{
			Type:      ErrorMsg,
			Content:   "Engine Error: " + msg.Error(),
			Sender:    "DuckOps",
			Timestamp: time.Now(),
		})
		return m, nil

	// ── Toast dismiss ───────────────────────────────────────────────
	case toastDismissMsg:
		m.toast = nil
		return m, nil

	// ── Window resize ───────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.wrappedCache = nil // Invalidate cache
		
		if m.stayAtBottom {
			m.scroll = 0
		}

		// Update textarea width to prevent weird text wrapping
		w := m.width - 6
		if m.showSidePanel {
			w -= components.SidePanelWidth
		}
		if w < 10 {
			w = 10
		}
		m.textarea.SetWidth(w)
		
		// Update sub-models that depend on size
		m.shell, _ = m.shell.Update(msg)
		return m, nil

	// ── Key events ──────────────────────────────────────────────────
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			m.scroll += 3
			m.stayAtBottom = false
		case tea.MouseWheelDown:
			if m.scroll > 0 {
				m.scroll -= 3
			}
			if m.scroll <= 0 {
				m.scroll = 0
				m.stayAtBottom = true
			}
		}
		return m, nil

	// ── Spinner tick ────────────────────────────────────────────────
	default:
		if m.loading {
			var spCmd tea.Cmd
			m.spinner, spCmd = m.spinner.Update(msg)
			cmds = append(cmds, spCmd)
		}
	}

	// Always update textarea.
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	cmds = append(cmds, taCmd)

	// Keep a fixed height for the textarea to ensure a stable layout
	m.textarea.SetHeight(3)

	// ── Sync Mode & Search (Works for both Keys and Paste) ────────
	val := m.textarea.Value()
	if strings.HasPrefix(val, "!") {
		if m.mode != ShellDiscoveryMode {
			m.mode = ShellDiscoveryMode
			m.discoveryIndex = 0
		}
		query := strings.TrimPrefix(val, "!")
		m.discoveryResults = m.discovery.Search(query)
		if m.discoveryIndex >= len(m.discoveryResults) {
			m.discoveryIndex = 0
		}
	} else if strings.HasPrefix(val, "@") {
		if m.mode != FileDiscoveryMode {
			m.mode = FileDiscoveryMode
			m.discoveryIndex = 0
		}
		query := strings.TrimPrefix(val, "@")
		m.discoveryResults = m.discovery.SearchFiles(query)
		if m.discoveryIndex >= len(m.discoveryResults) {
			m.discoveryIndex = 0
		}
	} else {
		m.mode = ChatMode
		m.discoveryResults = nil
	}

	// ── Command Menu Sync ──────────────────────────────────────────
	if m.showMenu && !strings.HasPrefix(val, "/") {
		m.showMenu = false
	}

	return m, tea.Batch(cmds...)
}

// ── Key handling ────────────────────────────────────────────────────

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 1. Global Interceptors
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.isProcessing && m.shell.active {
			// Interrupt the current shell process
			if m.shell.cmd != nil && m.shell.cmd.Process != nil {
				_ = m.shell.cmd.Process.Kill()
			}
			m.isProcessing = false
			m.loading = false
			m.toast = &Toast{Message: "Process interrupted", Level: ToastWarning}
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return toastDismissMsg{} })
		}
		return m, tea.Quit
	case tea.KeyEnter:
		input := strings.TrimSpace(m.textarea.Value())
		if strings.HasPrefix(input, "!") {
			return m.routeShellCommand(input)
		}
		// If shell is active but dead, normal enter should go back to chat
		if m.shellActive && !m.shell.active && input != "" {
			m.shellActive = false
			m.mode = ChatMode
		}
	case tea.KeyEsc:
		if m.shellActive {
			m.shellActive = false
			m.mode = ChatMode
			return m, nil
		}
	}

	// 2. Active View Delegation
	if m.shellActive {
		if msg.Type == tea.KeyCtrlF {
			m.shell.focused = !m.shell.focused
			return m, nil
		}
		
		if m.shell.focused {
			var cmd tea.Cmd
			m.shell, cmd = m.shell.Update(msg)
			return m, cmd
		} else {
			switch msg.Type {
			case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
				var cmd tea.Cmd
				m.shell, cmd = m.shell.Update(msg)
				return m, cmd
			case tea.KeyEsc:
				m.shellActive = false
				m.mode = ChatMode
				return m, nil
			}
		}
	}

	if m.activePopup != PopupNone {
		if msg.Type == tea.KeyEsc {
			m.activePopup = PopupNone
		}
		return m, nil
	}

	// ── Shell/File Discovery Mode ────────────────────────────────────
	if m.mode == ShellDiscoveryMode || m.mode == FileDiscoveryMode {
		switch msg.Type {
		case tea.KeyUp:
			if m.discoveryIndex > 0 {
				m.discoveryIndex--
			}
			return m, nil
		case tea.KeyDown:
			if m.discoveryIndex < len(m.discoveryResults)-1 {
				m.discoveryIndex++
			}
			return m, nil
		case tea.KeyTab:
			if len(m.discoveryResults) > 0 {
				prefix := "!"
				if m.mode == FileDiscoveryMode {
					prefix = "@"
				}
				selected := m.discoveryResults[m.discoveryIndex].Name
				m.textarea.SetValue(prefix + selected + " ")
				m.textarea.CursorEnd()
				return m, nil
			}
		}
	}

	// ── Menu mode ───────────────────────────────────────────────────
	if m.showMenu {
		return m.handleMenuKey(msg)
	}

	// ── Processing — only allow scroll + quit ───────────────────────
	if m.isProcessing {
		switch msg.Type {
		case tea.KeyUp:
			m.scroll++
			m.stayAtBottom = false
		case tea.KeyDown:
			if m.scroll > 0 {
				m.scroll--
			}
			if m.scroll == 0 {
				m.stayAtBottom = true
			}
		}
		return m, nil
	}

	// ── Normal mode keybindings ─────────────────────────────────────
	switch msg.Type {
	case tea.KeyEnter:
		if msg.Alt {
			// Alt+Enter: Insert a newline instead of sending
			var taCmd tea.Cmd
			m.textarea, taCmd = m.textarea.Update(msg)
			return m, taCmd
		}
		content := m.textarea.Value()
		if content != "" {
			if m.shellActive {
				m.shellActive = false
				m.mode = ChatMode
			}
			m.messages = append(m.messages, Message{
				Type:      UserMsg,
				Content:   content,
				Sender:    "You",
				Timestamp: time.Now(),
			})
			m.textarea.Reset()
			m.textarea.SetHeight(1)
			m.scroll = 0
			m.stayAtBottom = true
			m.isProcessing = true
			m.loading = true
			return m, tea.Batch(m.runAgent(content), m.spinner.Tick)
		}
		return m, nil

	case tea.KeyCtrlB:
		m.showSidePanel = !m.showSidePanel
		// Readjust textarea width
		w := m.width - 6
		if m.showSidePanel {
			w -= components.SidePanelWidth
		}
		if w < 10 {
			w = 10
		}
		m.textarea.SetWidth(w)
		return m, nil

	case tea.KeyEsc:
		if m.showMenu {
			m.showMenu = false
		}
		return m, nil

	case tea.KeyCtrlK:
		m.activePopup = PopupShortcuts
		return m, nil

	case tea.KeyUp:
		m.scroll += 3
		m.stayAtBottom = false
		return m, nil

	case tea.KeyDown:
		if m.scroll > 2 {
			m.scroll -= 3
		} else {
			m.scroll = 0
			m.stayAtBottom = true
		}
		return m, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 && msg.Runes[0] == '/' {
			if m.textarea.Value() == "" {
				m.showMenu = true
				m.menuSelection = 0
			}
		}
	}

	// Forward to textarea.
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)

	return m, taCmd
}

// ── Menu key handling ───────────────────────────────────────────────

func (m model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := components.GetFilteredMenuItems(m.textarea.Value())

	switch msg.Type {
	case tea.KeyEsc:
		m.showMenu = false
		return m, nil

	case tea.KeyUp:
		if m.menuSelection > 0 {
			m.menuSelection--
		}
		return m, nil

	case tea.KeyDown:
		if m.menuSelection < len(items)-1 {
			m.menuSelection++
		}
		return m, nil

	case tea.KeyEnter:
		if len(items) == 0 {
			// If nothing matches, just close the menu and let them keep typing.
			m.showMenu = false
			return m, nil
		}
		selectedCmd := items[m.menuSelection].Command
		m.showMenu = false
		m.messages = append(m.messages, Message{
			Type:      UserMsg,
			Content:   selectedCmd,
			Sender:    "You",
			Timestamp: time.Now(),
		})
		m.textarea.Reset()
		m.textarea.SetHeight(1)
		m.isProcessing = true
		m.loading = true
		return m, tea.Batch(m.runAgent(selectedCmd), m.spinner.Tick)
	}

	// Forward other keys (typing) to textarea
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)

	return m, taCmd
}

// ── Shell Routing ──────────────────────────────────────────────────

func (m *model) routeShellCommand(input string) (tea.Model, tea.Cmd) {
	rawCmd := strings.TrimPrefix(input, "!")
	if rawCmd == "" {
		return m, nil
	}

	parts, err := shlex.Split(rawCmd)
	if err != nil {
		m.toast = &Toast{Message: "Shell parse error: " + err.Error(), Level: ToastError}
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return toastDismissMsg{} })
	}
	if len(parts) == 0 {
		return m, nil
	}

	command := parts[0]
	args := parts[1:]

	// 1. Add User Command to Chat
	m.messages = append(m.messages, Message{
		Type:      UserMsg,
		Content:   input,
		Sender:    "You",
		Timestamp: time.Now(),
	})

	// 2. Add Placeholder Response from DuckOps with a nice Header
	header := " **Shell Mode**\n"
	m.messages = append(m.messages, Message{
		Type:      AgentMsg,
		Content:   header + "```shell\n```",
		Sender:    "DuckOps",
		Timestamp: time.Now(),
	})

	m.textarea.Reset()
	m.textarea.SetHeight(1)
	m.isProcessing = true
	m.loading = true
	m.scroll = 0
	m.stayAtBottom = true

	// Stay in ChatMode
	m.mode = ChatMode
	m.shellActive = false

	return m, m.shell.RunCommand(command, args)
}
