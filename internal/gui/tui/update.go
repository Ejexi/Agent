package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/subagent"
	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/engine"
	"github.com/SecDuckOps/agent/internal/gui/tui/components"
	plan_tools "github.com/SecDuckOps/agent/internal/tools/implementations/plan"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/shlex"
	"github.com/google/uuid"
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

// ── Helpers ─────────────────────────────────────────────────────────

// syncTextareaHeight updates the textarea height clamped to [1, 12].
func (m *model) syncTextareaHeight() {
	lines := calculateTextareaHeight(m.textarea)
	if lines > 12 {
		lines = 12
	} else if lines < 1 {
		lines = 1
	}
	m.textarea.SetHeight(lines)
}

// resetTextarea clears the textarea content and resets height to 1.
func (m *model) resetTextarea() {
	m.textarea.Reset()
	m.textarea.SetHeight(1)
}

// setTextareaWidth recalculates and applies the correct textarea width.
func (m *model) setTextareaWidth() {
	w := m.width - 6
	if m.showSidePanel {
		w -= components.SidePanelWidth
	}
	if w < 10 {
		w = 10
	}
	m.textarea.SetWidth(w)
}

// dismissPopup tears down any active ask-dialog / popup state.
func (m *model) dismissPopup() {
	m.askDialog = nil
	m.askChan = nil
	m.activePopup = PopupNone
}

// ── Agent command ───────────────────────────────────────────────────

func (m model) runAgent(userInput string) tea.Cmd {
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
func expandMentions(input string) string {
	words := strings.Fields(input)
	hasAt := false
	for _, w := range words {
		if strings.HasPrefix(w, "@") && len(w) > 1 {
			hasAt = true
			break
		}
	}
	if !hasAt {
		return input
	}

	cwd, _ := os.Getwd()
	var sb strings.Builder
	for i, word := range words {
		if i > 0 {
			sb.WriteByte(' ')
		}
		if !strings.HasPrefix(word, "@") || len(word) <= 1 {
			sb.WriteString(word)
			continue
		}

		filePath := strings.TrimPrefix(word, "@")
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(cwd, filePath)
		}

		info, err := os.Stat(filePath)
		if err != nil {
			sb.WriteString(word)
			continue
		}

		const maxSize = 100 * 1024
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
		fmt.Fprintf(&sb, "\n\n[Contents of %s]\n```%s\n%s\n```\n", filePath, ext, content)
	}
	return sb.String()
}

type agentStreamMsg struct{ ch <-chan any }

func waitForAgentEvent(ch <-chan any) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return nil
		}
		return evt
	}
}

// ── Update ──────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case ShellOutputMsg:
		if len(m.messages) > 0 {
			last := &m.messages[len(m.messages)-1]
			if last.Type == AgentMsg && last.Sender == "DuckOps" {
				content := last.Content
				if strings.HasSuffix(content, "```") {
					content = strings.TrimSuffix(content, "```")
				}
				if !strings.Contains(content, "```shell") {
					content += "```shell\n"
				}
				last.Content = content + stripANSI(string(msg)) + "```"
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
		content := msg.Message
		if msg.Type == subagent.EventThought {
			content = strings.TrimPrefix(content, "Thinking: ")
		}
		if msg.Type == subagent.EventPaused {
			var info *subagent.PauseInfo
			if mData, ok := msg.Data.(map[string]interface{}); ok {
				b, _ := json.Marshal(mData)
				_ = json.Unmarshal(b, &info)
			} else if pt, ok := msg.Data.(*subagent.PauseInfo); ok {
				info = pt
			}
			if info != nil && info.Reason == subagent.PauseToolApproval {
				m.activeSessionID = msg.SessionID
				var qb strings.Builder
				qb.WriteString("The agent wants to execute the following tool(s):\n")
				for _, ptc := range info.PendingToolCalls {
					argsStr := ""
					if b, err := json.MarshalIndent(ptc.Args, "", "  "); err == nil {
						argsStr = string(b)
					}
					fmt.Fprintf(&qb, "- %s\n%s\n", ptc.Name, argsStr)
				}
				qb.WriteString("\nDo you approve execution?")
				m.askDialog = components.NewAskUserDialog([]agent_domain.AskUserQuestion{
					{Header: "Security Gate", Question: qb.String(), Type: agent_domain.QuestionTypeYesNo},
				}, m.width)
				m.activePopup = PopupConfirm
				m.isToolApproval = true
				return m, nil
			}
		}
		m = appendAggregatedThought(m, content)
		return m, waitForAgentEvent(m.lastStreamCh)

	case engine.ThoughtStreamEvent:
		m = appendStreamedThought(m, msg.Chunk)
		return m, waitForAgentEvent(m.lastStreamCh)

	case engine.ThoughtEvent:
		// Only append if we didn't already stream it into the last thought msg
		if n := len(m.messages); n > 0 {
			last := &m.messages[n-1]
			if last.Type == ThoughtMsg && strings.HasSuffix(strings.TrimSpace(last.Content), strings.TrimSpace(msg.Rationale)) {
				return m, waitForAgentEvent(m.lastStreamCh) // Already streamed
			}
		}
		m = appendAggregatedThought(m, msg.Rationale)
		return m, waitForAgentEvent(m.lastStreamCh)

	case engine.ReflectionEvent:
		m = appendAggregatedThought(m, msg.Reflection)
		return m, waitForAgentEvent(m.lastStreamCh)

	case agent_domain.AskUserEvent:
		m.askDialog = components.NewAskUserDialog(msg.Questions, m.width)
		m.askChan = msg.ResponseChan
		m.activePopup = PopupConfirm
		return m, waitForAgentEvent(m.lastStreamCh)

	case agent_domain.PlanModeChangedEvent:
		m.inPlanMode = msg.Active
		status := "Exited plan mode"
		if msg.Active {
			status = "Entered plan mode"
		}
		if msg.Reason != "" {
			status += fmt.Sprintf(" (%s)", msg.Reason)
		}
		m.toast = &Toast{Message: status, Level: ToastInfo}
		return m, tea.Batch(
			waitForAgentEvent(m.lastStreamCh),
			tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastDismissMsg{} }),
		)

	case plan_tools.PlanReadyEvent:
		respCh := make(chan agent_domain.AskUserResponse, 1)
		m.askChan = respCh
		m.askDialog = components.NewAskUserDialog([]agent_domain.AskUserQuestion{
			{
				Header:   "Plan Ready",
				Question: fmt.Sprintf("Review plan at %s\nApprove execution?", msg.PlanPath),
				Type:     agent_domain.QuestionTypeYesNo,
			},
		}, m.width)
		m.activePopup = PopupConfirm
		go func() {
			ans := <-respCh
			msg.ResponseChan <- plan_tools.PlanApproval{Approved: isYes(ans)}
		}()
		return m, waitForAgentEvent(m.lastStreamCh)

	case agent_domain.SkillActivationEvent:
		respCh := make(chan agent_domain.AskUserResponse, 1)
		m.askChan = respCh
		m.askDialog = components.NewAskUserDialog([]agent_domain.AskUserQuestion{
			{
				Header:   "Skill Activation",
				Question: fmt.Sprintf("Activate skill %s?\n%s", msg.SkillName, msg.Description),
				Type:     agent_domain.QuestionTypeYesNo,
			},
		}, m.width)
		m.activePopup = PopupConfirm
		go func() {
			ans := <-respCh
			msg.ResponseChan <- isYes(ans)
		}()
		return m, waitForAgentEvent(m.lastStreamCh)

	case engine.ChatResult:
		m.isProcessing = false
		m.loading = false
		m.messages = append(m.messages, Message{
			Type:       AgentMsg,
			Content:    msg.Content,
			Sender:     "DuckOps",
			Timestamp:  time.Now(),
			Checkpoint: uuid.New().String()[:8], // Shorter UUID for cleaner UI
		})
		m.totalUsage.PromptTokens += msg.Usage.PromptTokens
		m.totalUsage.CompletionTokens += msg.Usage.CompletionTokens
		m.totalUsage.TotalTokens += msg.Usage.TotalTokens
		if msg.Model != "" {
			m.activeModel = msg.Model
		}
		m.scroll = 0
		m.stayAtBottom = true
		return m.dequeueNext()

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
		return m.dequeueNext()

	case error:
		m.isProcessing = false
		m.loading = false
		m.messages = append(m.messages, Message{
			Type:      ErrorMsg,
			Content:   "Engine Error: " + msg.Error(),
			Sender:    "DuckOps",
			Timestamp: time.Now(),
		})
		return m.dequeueNext()

	case toastDismissMsg:
		m.toast = nil
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.wrappedCache = nil
		if m.stayAtBottom {
			m.scroll = 0
		}
		m.setTextareaWidth()
		m.shell, _ = m.shell.Update(msg)
		return m, nil

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

	default:
		if m.loading {
			var spCmd tea.Cmd
			m.spinner, spCmd = m.spinner.Update(msg)
			cmds = append(cmds, spCmd)
		}
	}

	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	cmds = append(cmds, taCmd)
	m.syncTextareaHeight()
	m.syncDiscoveryMode()

	if m.showMenu && !strings.HasPrefix(m.textarea.Value(), "/") {
		m.showMenu = false
	}

	return m, tea.Batch(cmds...)
}

// syncDiscoveryMode updates mode and results based on current textarea value.
func (m *model) syncDiscoveryMode() {
	val := m.textarea.Value()
	if strings.HasPrefix(val, "!") {
		if m.mode != ShellDiscoveryMode {
			m.mode = ShellDiscoveryMode
			m.discoveryIndex = 0
		}
		m.discoveryResults = m.discovery.Search(strings.TrimPrefix(val, "!"))
		if m.discoveryIndex >= len(m.discoveryResults) {
			m.discoveryIndex = 0
		}
	} else if atQuery := extractAtQuery(val); atQuery != "" {
		if m.mode != FileDiscoveryMode {
			m.mode = FileDiscoveryMode
			m.discoveryIndex = 0
		}
		m.discoveryResults = m.discovery.SearchFiles(atQuery)
		if m.discoveryIndex >= len(m.discoveryResults) {
			m.discoveryIndex = 0
		}
	} else if m.mode == FileDiscoveryMode || m.mode == ShellDiscoveryMode {
		m.mode = ChatMode
		m.discoveryResults = nil
	}
}

// ── Key handling ────────────────────────────────────────────────────

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global interceptors
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.isProcessing && m.shell.active {
			if m.shell.cmd != nil && m.shell.cmd.Process != nil {
				_ = m.shell.cmd.Process.Kill()
			}
			m.isProcessing = false
			m.loading = false
			m.toast = &Toast{Message: "Process interrupted", Level: ToastWarning}
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return toastDismissMsg{} })
		}
		if m.isExitPrompt {
			return m, tea.Quit
		}
		m.isExitPrompt = true
		m.askDialog = components.NewAskUserDialog([]agent_domain.AskUserQuestion{
			{Header: "Exit", Question: "Are you sure you want to exit DuckOps?", Type: agent_domain.QuestionTypeYesNo},
		}, m.width)
		m.activePopup = PopupConfirm
		return m, nil

	case tea.KeyEnter:
		input := strings.TrimSpace(m.textarea.Value())
		if strings.HasPrefix(input, "!") {
			return m.routeShellCommand(input)
		}
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

	// Shell active delegation
	if m.shellActive {
		if msg.Type == tea.KeyCtrlF {
			m.shell.focused = !m.shell.focused
			return m, nil
		}
		if m.shell.focused {
			var cmd tea.Cmd
			m.shell, cmd = m.shell.Update(msg)
			return m, cmd
		}
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

	// Popup delegation
	if m.activePopup != PopupNone {
		if m.activePopup == PopupConfirm && m.askDialog != nil {
			return m.handleConfirmKey(msg)
		}
		if msg.Type == tea.KeyEsc {
			m.activePopup = PopupNone
		}
		return m, nil
	}

	// Discovery navigation
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
					if atIdx := strings.LastIndex(current, "@"); atIdx >= 0 {
						m.textarea.SetValue(current[:atIdx] + "@" + selected + " ")
					} else {
						m.textarea.SetValue("@" + selected + " ")
					}
				} else {
					m.textarea.SetValue("!" + selected + " ")
				}
				m.textarea.CursorEnd()
			}
			return m, nil
		}
	}

	if m.showMenu {
		return m.handleMenuKey(msg)
	}

	// Processing mode — allow scroll + queue input
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
			if input := strings.TrimSpace(m.textarea.Value()); input != "" {
				m.queuedMessages = append(m.queuedMessages, input)
				m.messages = append(m.messages, Message{
					Type:      UserMsg,
					Content:   input + " (Queued...)",
					Sender:    "You",
					Timestamp: time.Now(),
				})
				m.resetTextarea()
				m.scroll = 0
				m.stayAtBottom = true
			}
			return m, nil
		}
		// Other keys fall through to textarea
	}

	// Normal mode
	switch msg.Type {
	case tea.KeyEnter:
		if msg.Alt {
			var taCmd tea.Cmd
			m.textarea, taCmd = m.textarea.Update(msg)
			return m, taCmd
		}
		if content := m.textarea.Value(); content != "" {
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
			m.resetTextarea()
			m.scroll = 0
			m.stayAtBottom = true

			switch strings.TrimSpace(strings.ToLower(content)) {
			case "list all tools", "/tools":
				return m.showToolsTable()
			case "list all skills", "/skills":
				return m.showSkillsTable()
			}

			m.isProcessing = true
			m.loading = true
			return m, tea.Batch(m.runAgent(content), m.spinner.Tick)
		}
		return m, nil

	case tea.KeyCtrlB:
		m.showSidePanel = !m.showSidePanel
		m.setTextareaWidth()
		return m, nil

	case tea.KeyEsc:
		m.showMenu = false
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
		if len(msg.Runes) == 1 && msg.Runes[0] == '/' && m.textarea.Value() == "" {
			m.showMenu = true
			m.menuSelection = 0
		}
	}

	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	m.syncTextareaHeight()
	return m, taCmd
}

// handleConfirmKey handles keyboard input while an ask-dialog popup is active.
func (m model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.askDialog.MoveUp()
	case tea.KeyDown:
		m.askDialog.MoveDown()
	case tea.KeyTab:
		m.askDialog.NextQuestion()
	case tea.KeyShiftTab:
		m.askDialog.PrevQuestion()
	case tea.KeySpace:
		m.askDialog.ToggleMultiSelect()
	case tea.KeyBackspace, tea.KeyDelete:
		m.askDialog.Backspace()
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.askDialog.TypeChar(r)
		}

	case tea.KeyEsc:
		if m.askChan != nil {
			m.askChan <- agent_domain.AskUserResponse{Cancelled: true}
		}
		if m.isToolApproval {
			m.resumeSession(false)
		}
		m.dismissPopup()
		m.isToolApproval = false
		if m.isExitPrompt {
			m.isExitPrompt = false
			return m, nil
		}
		return m, waitForAgentEvent(m.lastStreamCh)

	case tea.KeyEnter:
		ans := m.askDialog.Collect()
		if m.isExitPrompt {
			if ans[0] == "yes" || ans[0] == "Yes" {
				return m, tea.Quit
			}
			m.dismissPopup()
			m.isExitPrompt = false
			return m, nil
		}
		if m.isToolApproval {
			m.resumeSession(ans[0] == "yes" || ans[0] == "Yes")
			m.askDialog = nil
			m.activePopup = PopupNone
			m.isToolApproval = false
			return m, waitForAgentEvent(m.lastStreamCh)
		}
		if m.askChan != nil {
			m.askChan <- agent_domain.AskUserResponse{Answers: ans}
		}
		m.dismissPopup()
		return m, waitForAgentEvent(m.lastStreamCh)
	}
	return m, nil
}

// resumeSession approves or rejects a tool-approval session.
func (m *model) resumeSession(approved bool) {
	if m.sessionManager != nil && m.activeSessionID != "" {
		_ = m.sessionManager.ResumeSession(m.activeSessionID, subagent.ResumeDecision{
			ApproveAll: approved,
			RejectAll:  !approved,
		})
	}
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
		m.resetTextarea()
		return m.execMenuCommand(selectedCmd)
	}

	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	m.syncTextareaHeight()
	return m, taCmd
}

// execMenuCommand dispatches a slash command chosen from the menu.
func (m model) execMenuCommand(cmd string) (tea.Model, tea.Cmd) {
	switch cmd {
	case "/clear":
		m.messages = nil
		return m, nil
	case "/tools":
		return m.showToolsTable()
	case "/skills":
		return m.showSkillsTable()
	case "/help":
		m.messages = append(m.messages,
			Message{Type: UserMsg, Content: cmd, Sender: "You", Timestamp: time.Now()},
			Message{Type: AgentMsg, Content: helpSlashContent(), Sender: "DuckOps", Timestamp: time.Now()},
		)
		return m, nil
	case "/exit":
		return m, tea.Quit
	default:
		// /status, /scan, /vuln, or any unknown slash command → agent
		agentPrompts := map[string]string{
			"/status": "What is the current system status? Check Docker availability and list any active sessions.",
			"/scan":   "Run a full security scan on the current project directory.",
			"/vuln":   "Show me all known vulnerabilities and security findings for this project.",
		}
		prompt, ok := agentPrompts[cmd]
		if !ok {
			prompt = cmd
		}
		m.messages = append(m.messages, Message{Type: UserMsg, Content: cmd, Sender: "You", Timestamp: time.Now()})
		m.isProcessing = true
		m.loading = true
		return m, tea.Batch(m.runAgent(prompt), m.spinner.Tick)
	}
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
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastDismissMsg{} })
	}
	if len(parts) == 0 {
		return m, nil
	}

	m.messages = append(m.messages,
		Message{Type: UserMsg, Content: input, Sender: "You", Timestamp: time.Now()},
		Message{Type: AgentMsg, Content: " **Shell Mode**\n```shell\n```", Sender: "DuckOps", Timestamp: time.Now()},
	)
	m.resetTextarea()
	m.isProcessing = true
	m.loading = true
	m.scroll = 0
	m.stayAtBottom = true
	m.mode = ChatMode
	m.shellActive = false

	return m, m.shell.RunCommand(parts[0], parts[1:])
}

// ── Tools / Skills Display ─────────────────────────────────────────

func (m *model) showToolsTable() (tea.Model, tea.Cmd) {
	var rows [][]string
	for i, t := range m.engine.GetAvailableTools() {
		rows = append(rows, []string{fmt.Sprintf("%d", i+1), t.Name, t.Description})
	}
	m.messages = append(m.messages, Message{
		Type:         AgentMsg,
		Content:      "Here's the complete list of tools available to me:\n\n**File, Execution & Security Operations**",
		Sender:       "DuckOps",
		Timestamp:    time.Now(),
		TableHeaders: []string{"#", "Tool", "Description"},
		TableData:    rows,
	})
	m.scroll = 0
	m.stayAtBottom = true
	return m, nil
}

func (m *model) showSkillsTable() (tea.Model, tea.Cmd) {
	var rows [][]string
	if m.skillRegistry != nil {
		for _, s := range m.skillRegistry.ListSkills() {
			rows = append(rows, []string{s.Name, s.Description})
		}
	} else {
		rows = [][]string{{"Error", "Skill registry not initialized."}}
	}
	m.messages = append(m.messages, Message{
		Type:         AgentMsg,
		Content:      "Here's the complete list of Knowledge Skills loaded in my memory. I can dynamically fetch their full content when needed:\n\n**Available Agent Skills**",
		Sender:       "DuckOps",
		Timestamp:    time.Now(),
		TableHeaders: []string{"Skill Name", "Focus Area / Description"},
		TableData:    rows,
	})
	m.scroll = 0
	m.stayAtBottom = true
	return m, nil
}

// ── Utilities ───────────────────────────────────────────────────────

// extractAtQuery returns the query string after the last '@' token if it has no trailing space.
func extractAtQuery(input string) string {
	atIdx := strings.LastIndex(input, "@")
	if atIdx < 0 {
		return ""
	}
	after := input[atIdx+1:]
	if strings.Contains(after, " ") {
		return ""
	}
	return after
}

// isYes reports whether an AskUserResponse represents an affirmative answer.
func isYes(ans agent_domain.AskUserResponse) bool {
	if ans.Cancelled || len(ans.Answers) == 0 {
		return false
	}
	v := strings.ToLower(ans.Answers[0])
	return v == "yes" || v == "y"
}

// dequeueNext starts processing the next queued message, if any.
func (m model) dequeueNext() (tea.Model, tea.Cmd) {
	if len(m.queuedMessages) == 0 {
		return m, nil
	}
	next := m.queuedMessages[0]
	m.queuedMessages = m.queuedMessages[1:]
	m.isProcessing = true
	m.loading = true
	m.scroll = 0
	m.stayAtBottom = true

	if strings.HasPrefix(next, "!") {
		return m.routeShellCommand(next)
	}
	return m, tea.Batch(m.runAgent(next), m.spinner.Tick)
}

// appendAggregatedThought appends or merges a thought line into the message list.
func appendAggregatedThought(m model, content string) model {
	text := "- " + strings.ReplaceAll(strings.TrimSpace(content), "\n", "\n  ")
	if n := len(m.messages); n > 0 {
		last := &m.messages[n-1]
		if last.Type == ThoughtMsg || last.Type == LearningMsg || last.Type == ReflectionMsg {
			last.Type = ThoughtMsg
			last.Content += "\n" + text
			m.scroll = 0
			m.stayAtBottom = true
			return m
		}
	}
	m.messages = append(m.messages, Message{
		Type:      ThoughtMsg,
		Content:   text,
		Sender:    "DuckOps",
		Timestamp: time.Now(),
	})
	m.scroll = 0
	m.stayAtBottom = true
	return m
}

// appendStreamedThought appends a raw streamed chunk without bullet formatting.
func appendStreamedThought(m model, chunk string) model {
	if n := len(m.messages); n > 0 {
		last := &m.messages[n-1]
		if last.Type == ThoughtMsg || last.Type == LearningMsg || last.Type == ReflectionMsg {
			last.Type = ThoughtMsg
			last.Content += chunk
			m.scroll = 0
			m.stayAtBottom = true
			return m
		}
	}
	m.messages = append(m.messages, Message{
		Type:      ThoughtMsg,
		Content:   chunk,
		Sender:    "DuckOps",
		Timestamp: time.Now(),
	})
	m.scroll = 0
	m.stayAtBottom = true
	return m
}

// calculateTextareaHeight returns the number of visual lines the textarea content occupies.
func calculateTextareaHeight(ta textarea.Model) int {
	w := ta.Width()
	if w <= 0 {
		w = 80
	}
	total := 0
	for _, line := range strings.Split(ta.Value(), "\n") {
		lw := lipgloss.Width(line)
		if lw == 0 {
			total++
			continue
		}
		total += lw / w
		if lw%w != 0 {
			total++
		}
	}
	if total < 1 {
		return 1
	}
	return total
}

// helpSlashContent returns the markdown help text for /help.
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