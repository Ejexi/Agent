package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	// Expand @file mentions → inject file content into the prompt
	expanded := expandMentions(userInput)
	return func() tea.Msg {
		ch, err := m.engine.StreamChat(context.Background(), expanded)
		if err != nil {
			return agentResponseMsg{err: err}
		}
		return agentStreamMsg{ch: ch}
	}
}

// expandMentions replaces @path/to/file tokens with the file's content.
// Supports: @file.go  @./relative/path  @/absolute/path
// Files that don't exist or are too large (>100KB) are left as-is with a note.
func expandMentions(input string) string {
	words := strings.Fields(input)
	hasAtMention := false
	for _, w := range words {
		if strings.HasPrefix(w, "@") && len(w) > 1 {
			hasAtMention = true
			break
		}
	}
	if !hasAtMention {
		return input
	}

	var sb strings.Builder
	for i, word := range words {
		if i > 0 {
			sb.WriteString(" ")
		}
		if !strings.HasPrefix(word, "@") || len(word) <= 1 {
			sb.WriteString(word)
			continue
		}

		filePath := strings.TrimPrefix(word, "@")

		// Resolve relative paths from cwd
		if !filepath.IsAbs(filePath) {
			cwd, _ := os.Getwd()
			filePath = filepath.Join(cwd, filePath)
		}

		info, err := os.Stat(filePath)
		if err != nil {
			// File not found — keep token, let agent handle it
			sb.WriteString(word)
			continue
		}

		const maxSize = 100 * 1024 // 100KB
		if info.Size() > maxSize {
			sb.WriteString(fmt.Sprintf("[%s: file too large to inline (%d bytes)]", word, info.Size()))
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			sb.WriteString(word)
			continue
		}

		ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
		sb.WriteString(fmt.Sprintf("\n\n[Contents of %s]\n```%s\n%s\n```\n", filePath, ext, string(content)))
	}
	return sb.String()
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
		msgType := LearningMsg
		content := msg.Message

		if msg.Type == subagent.EventThought {
			msgType = ThoughtMsg
			content = strings.TrimPrefix(content, "Thinking: ")
		}

		m.messages = append(m.messages, Message{
			Type:      msgType,
			Content:   content,
			Sender:    "DuckOps",
			Timestamp: msg.Timestamp,
		})
		m.scroll = 0
		m.stayAtBottom = true
		return m, waitForAgentEvent(m.lastStreamCh)

	case engine.ThoughtEvent:
		m.messages = append(m.messages, Message{
			Type:      ThoughtMsg,
			Content:   msg.Rationale,
			Sender:    "DuckOps",
			Timestamp: time.Now(),
		})
		m.scroll = 0
		m.stayAtBottom = true
		return m, waitForAgentEvent(m.lastStreamCh)

	case engine.ReflectionEvent:
		m.messages = append(m.messages, Message{
			Type:      ReflectionMsg,
			Content:   msg.Reflection,
			Sender:    "DuckOps",
			Timestamp: time.Now(),
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
		m.wrappedCache = nil

		if m.stayAtBottom {
			m.scroll = 0
		}

		w := m.width - 6
		if m.showSidePanel {
			w -= components.SidePanelWidth
		}
		if w < 10 {
			w = 10
		}
		m.textarea.SetWidth(w)
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
	} else if atQuery := extractAtQuery(val); atQuery != "" {
		// @ can appear anywhere in the input: "review @file.go" works too
		if m.mode != FileDiscoveryMode {
			m.mode = FileDiscoveryMode
			m.discoveryIndex = 0
		}
		m.discoveryResults = m.discovery.SearchFiles(atQuery)
		if m.discoveryIndex >= len(m.discoveryResults) {
			m.discoveryIndex = 0
		}
	} else {
		if m.mode == FileDiscoveryMode || m.mode == ShellDiscoveryMode {
			m.mode = ChatMode
			m.discoveryResults = nil
		}
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
				selected := m.discoveryResults[m.discoveryIndex].Name
				current := m.textarea.Value()

				if m.mode == FileDiscoveryMode {
					// Replace only the @query token, preserve rest of the input
					atIdx := strings.LastIndex(current, "@")
					if atIdx >= 0 {
						before := current[:atIdx]
						m.textarea.SetValue(before + "@" + selected + " ")
					} else {
						m.textarea.SetValue("@" + selected + " ")
					}
				} else {
					// Shell (!command) mode — replace whole input
					m.textarea.SetValue("!" + selected + " ")
				}
				m.textarea.CursorEnd()
				return m, nil
			}
		}
	}

	// ── Menu mode ───────────────────────────────────────────────────
	if m.showMenu {
		return m.handleMenuKey(msg)
	}

	// ── Processing — allow scroll, but don't block input ──────────────
	if m.isProcessing {
		switch msg.Type {
		case tea.KeyUp:
			m.scroll++
			m.stayAtBottom = false
			return m, nil
		case tea.KeyDown:
			if m.scroll > 0 {
				m.scroll--
			}
			if m.scroll == 0 {
				m.stayAtBottom = true
			}
			return m, nil
		case tea.KeyEnter:
			// Prevent sending a new message while the agent is already processing one
			m.toast = &Toast{Message: "Agent is currently processing a request", Level: ToastWarning}
			return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return toastDismissMsg{} })
		}
		// We purposefully DO NOT return here for other keys, 
		// allowing them to fall through to the textarea update at the end!
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

			// Intercept specific tool list query before hitting Agent processing state
			query := strings.TrimSpace(strings.ToLower(content))
			if query == "list all tools" || query == "/tools" {
				return m.showToolsTable()
			}
			if query == "list all skills" || query == "/skills" {
				return m.showSkillsTable()
			}

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
			m.showMenu = false
			return m, nil
		}
		selectedCmd := items[m.menuSelection].Command
		m.showMenu = false
		m.textarea.Reset()
		m.textarea.SetHeight(1)

		// Handle slash commands locally before hitting the agent
		switch selectedCmd {
		case "/clear":
			m.messages = nil
			return m, nil

		case "/help":
			m.messages = append(m.messages, Message{
				Type:      UserMsg,
				Content:   selectedCmd,
				Sender:    "You",
				Timestamp: time.Now(),
			})
			m.messages = append(m.messages, Message{
				Type:      AgentMsg,
				Content:   helpSlashContent(),
				Sender:    "DuckOps",
				Timestamp: time.Now(),
			})
			return m, nil

		case "/status":
			m.messages = append(m.messages, Message{
				Type:      UserMsg,
				Content:   selectedCmd,
				Sender:    "You",
				Timestamp: time.Now(),
			})
			m.isProcessing = true
			m.loading = true
			return m, tea.Batch(m.runAgent("What is the current system status? Check Docker availability and list any active sessions."), m.spinner.Tick)

		case "/scan":
			m.messages = append(m.messages, Message{
				Type:      UserMsg,
				Content:   selectedCmd,
				Sender:    "You",
				Timestamp: time.Now(),
			})
			m.isProcessing = true
			m.loading = true
			return m, tea.Batch(m.runAgent("Run a full security scan on the current project directory."), m.spinner.Tick)

		case "/vuln":
			m.messages = append(m.messages, Message{
				Type:      UserMsg,
				Content:   selectedCmd,
				Sender:    "You",
				Timestamp: time.Now(),
			})
			m.isProcessing = true
			m.loading = true
			return m, tea.Batch(m.runAgent("Show me all known vulnerabilities and security findings for this project."), m.spinner.Tick)

		case "/logout":
			return m, tea.Quit

		default:
			// Unknown slash command → send to agent as natural language
			m.messages = append(m.messages, Message{
				Type:      UserMsg,
				Content:   selectedCmd,
				Sender:    "You",
				Timestamp: time.Now(),
			})
			m.isProcessing = true
			m.loading = true
			return m, tea.Batch(m.runAgent(selectedCmd), m.spinner.Tick)
		}
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

// ── Tools Display ──────────────────────────────────────────────────

func (m *model) showToolsTable() (tea.Model, tea.Cmd) {
	toolsHeaders := []string{"#", "Tool", "Description"}
	
	// Mimic exactly what the user requested ("File & Code Operations" then "Command Execution")
	toolsData := [][]string{
		{"1", "view_file", "Read files/directories, supports glob, line ranges, tree view"},
		{"2", "write_to_file", "Create or overwrite files with code content (local machine)"},
		{"3", "multi_replace_file_content", "Find & replace exact text in files robustly"},
		{"4", "grep_search", "Deep grep search across the codebase"},
		{"5", "list_dir", "List files and subdirectories structurally"},
		{"6", "run_command", "Execute shell commands securely in DuckOps workspace"},
		{"7", "docker_warden", "Spins up ephemeral sandboxed Docker containers"},
		{"8", "sast_scanner", "Runs targeted SAST (Semgrep, Trivy, Gosec) against codebase"},
	}

	m.messages = append(m.messages, Message{
		Type:         AgentMsg,
		Content:      "Here's the complete list of tools available to me:\n\n**File, Execution & Security Operations**",
		Sender:       "DuckOps",
		Timestamp:    time.Now(),
		TableHeaders: toolsHeaders,
		TableData:    toolsData,
	})

	m.scroll = 0
	m.stayAtBottom = true

	return m, nil
}

// ── Skills Display ──────────────────────────────────────────────────

func (m *model) showSkillsTable() (tea.Model, tea.Cmd) {
	skillsHeaders := []string{"Skill Name", "Focus Area / Description"}
	
	var skillsData [][]string
	if m.skillRegistry != nil {
		available := m.skillRegistry.ListSkills()
		for _, s := range available {
			skillsData = append(skillsData, []string{s.Name, s.Description})
		}
	} else {
		skillsData = [][]string{{"Error", "Skill registry not initialized."}}
	}

	m.messages = append(m.messages, Message{
		Type:         AgentMsg,
		Content:      "Here's the complete list of Knowledge Skills loaded in my memory. I can dynamically fetch their full content when needed:\n\n**Available Agent Skills**",
		Sender:       "DuckOps",
		Timestamp:    time.Now(),
		TableHeaders: skillsHeaders,
		TableData:    skillsData,
	})

	m.scroll = 0
	m.stayAtBottom = true

	return m, nil
}

// extractAtQuery finds the last @token in the input and returns the query after @.
// Returns "" if no active @mention is found.
// e.g. "review @main" → "main",  "hello world" → ""
func extractAtQuery(input string) string {
	atIdx := strings.LastIndex(input, "@")
	if atIdx < 0 {
		return ""
	}
	after := input[atIdx+1:]
	// Must be contiguous (no space after @)
	if strings.Contains(after, " ") {
		return ""
	}
	return after
}

// helpSlashContent returns the help content for /help slash command.
func helpSlashContent() string {
	return `## 🦆 DuckOps Commands

### Slash Commands
| Command | Action |
|---------|--------|
| ` + "`/scan`" + ` | Full security scan on current project |
| ` + "`/vuln`" + ` | Show all known vulnerabilities |
| ` + "`/status`" + ` | System + Docker status |
| ` + "`/clear`" + ` | Clear conversation |
| ` + "`/tools`" + ` | List all available tools |
| ` + "`/skills`" + ` | List all loaded skills |
| ` + "`/logout`" + ` | Sign out |

### @ File Mentions
Type ` + "`@`" + ` followed by a filename to include its content in your message.
Use **Tab** to autocomplete from the fuzzy search results.

` + "```" + `
@main.go review this file for security issues
@./internal/auth/handler.go what does this do?
` + "```" + `

### ! Shell Commands
Type ` + "`!`" + ` followed by any shell command to run it directly.

` + "```" + `
!ls -la
!git log --oneline -10
!docker ps
` + "```" + `

### Keyboard Shortcuts
| Key | Action |
|-----|--------|
| ` + "`Enter`" + ` | Send message |
| ` + "`Alt+Enter`" + ` | New line |
| ` + "`Ctrl+C`" + ` | Quit / interrupt |
| ` + "`Ctrl+B`" + ` | Toggle side panel |
| ` + "`↑ / ↓`" + ` | Scroll messages |
`
}



