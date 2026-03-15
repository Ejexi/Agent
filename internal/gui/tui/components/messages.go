package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/muesli/reflow/wordwrap"
)

// ── Markdown Rendering ─────────────────────────────────────────────

func renderMarkdown(content string, width int) string {
	// Initialize a glamour renderer with a refined style
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		// Fallback to simple word wrap if glamour fails
		return wordwrap.String(content, width)
	}

	out, err := r.Render(content)
	if err != nil {
		return wordwrap.String(content, width)
	}

	return strings.TrimSpace(out)
}

// TableRow represents a row in the status table.
type TableRow struct {
	Check  string
	Status string // "success", "warning", "failed"
	Impact string
}

// ChatMessage is the component-level representation of a message.
type ChatMessage struct {
	Type        int // 0=User, 1=Agent, 2=System, 3=Error
	Content     string
	Sender      string
	Timestamp   time.Time
	Table       []TableRow
	Suggestions []string
}

// ChatMessage type constants (mirror model-level enum w/o import cycle).
const (
	ChatUser   = 0
	ChatAgent  = 1
	ChatSystem = 2
	ChatError  = 3
)

// ── Adaptive message colours ────────────────────────────────────────

var (
	userMsgAccent = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}
	agentAccent   = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#34D399"}
	systemAccent  = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	errorAccent   = lipgloss.AdaptiveColor{Light: "#D32F2F", Dark: "#FF6B6B"}
	textColor     = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	mutedColor    = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
)

// RenderMessageArea renders the full messages area at the given dimensions.
// `scrollOffset` is measured from the bottom (0 = at bottom).
func RenderMessageArea(msgs []ChatMessage, width, height, scrollOffset int, logo string, suggestions []string) string {
	if width < 4 {
		width = 4
	}

	// Render all messages and collect lines.
	var allLines []string
	for _, msg := range msgs {
		rendered := renderSingleMessage(msg, width-4) // 4 = left/right margins
		lines := strings.Split(rendered, "\n")
		allLines = append(allLines, lines...)
	}

	// Empty state
	if len(allLines) == 0 {
		return renderEmptyState(width, height, logo, suggestions)
	}

	// Apply scroll: take the slice of lines visible in the viewport.
	totalLines := len(allLines)
	if scrollOffset > totalLines-height && totalLines > height {
		scrollOffset = totalLines - height
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	end := totalLines - scrollOffset
	if end < 0 {
		end = 0
	}
	start := end - height
	if start < 0 {
		start = 0
	}

	visible := allLines[start:end]

	// Pad with empty lines to fill the full height.
	for len(visible) < height {
		visible = append([]string{""}, visible...)
	}

	content := strings.Join(visible, "\n")
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(content)
}

// MaxScroll returns the maximum scroll offset for the given messages.
func MaxScroll(msgs []ChatMessage, width, height int) int {
	var totalLines int
	for _, msg := range msgs {
		rendered := renderSingleMessage(msg, width-4)
		totalLines += strings.Count(rendered, "\n") + 1
	}
	max := totalLines - height
	if max < 0 {
		return 0
	}
	return max
}

// ── Single message rendering ────────────────────────────────────────

func renderSingleMessage(msg ChatMessage, contentWidth int) string {
	if contentWidth < 10 {
		contentWidth = 10
	}

	var body string
	if msg.Type == ChatAgent || msg.Type == ChatError {
		body = renderMarkdown(msg.Content, contentWidth-4) // -4 for indicator bar + padding
	} else {
		body = wordwrap.String(msg.Content, contentWidth-4)
	}

	var accent lipgloss.AdaptiveColor
	switch msg.Type {
	case ChatUser:
		accent = userMsgAccent
	case ChatAgent:
		accent = agentAccent
	case ChatSystem:
		accent = systemAccent
	case ChatError:
		accent = errorAccent
	default:
		accent = textColor
	}

	// Detect if this is purely a shell output (no prose)
	isPureShell := strings.HasPrefix(msg.Content, "```shell") && strings.HasSuffix(strings.TrimSpace(msg.Content), "```")

	// Header: sender + timestamp
	senderStyle := lipgloss.NewStyle().Foreground(accent).Bold(true)
	timeStyle := lipgloss.NewStyle().Foreground(mutedColor).Italic(true)
	timeStr := msg.Timestamp.Format("15:04")
	sentCue := ""
	if msg.Type == ChatUser {
		sentCue = " " + lipgloss.NewStyle().Foreground(accent).Render("✓")
	}
	header := fmt.Sprintf("%s  %s%s", senderStyle.Render(msg.Sender), timeStyle.Render(timeStr), sentCue)

	var renderedBody string
	if isPureShell {
		// Render without indicator for shell blocks to preserve alignment.
		// Strip the ```shell and ``` markers for cleaner raw-like display
		cleanContent := strings.TrimSpace(msg.Content)
		cleanContent = strings.TrimPrefix(cleanContent, "```shell")
		cleanContent = strings.TrimSuffix(cleanContent, "```")
		cleanContent = strings.TrimSpace(cleanContent)
		
		// Use simple wordwrap or just keep it as is if it fits
		renderedBody = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2).
			Render(cleanContent)
	} else {
		// Status line indicator (Glow style) for normal messages
		indicatorStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(accent).
			PaddingLeft(2)
		renderedBody = indicatorStyle.Render(body)
	}

	res := lipgloss.JoinVertical(lipgloss.Left, header, renderedBody)

	// Render table/suggestions if present
	if len(msg.Table) > 0 {
		tbl := renderTable(msg.Table, contentWidth)
		res = lipgloss.JoinVertical(lipgloss.Left, res, "", tbl)
	}
	if len(msg.Suggestions) > 0 {
		suggs := renderSuggestions(msg.Suggestions, contentWidth)
		res = lipgloss.JoinVertical(lipgloss.Left, res, "", suggs)
	}

	return lipgloss.JoinVertical(lipgloss.Left, res, "")
}

func renderTable(rows []TableRow, width int) string {
	if len(rows) == 0 {
		return ""
	}

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}).Bold(true)
	borderStyle := lipgloss.NewStyle().Foreground(mutedColor)

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		Headers("Check", "Status", "Impact").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return headerStyle.Padding(0, 1) // Header row
			}
			return lipgloss.NewStyle().Padding(0, 1) // Data rows
		})

	for _, r := range rows {
		statusStr := renderStatusBadge(r.Status)
		tbl.Row(r.Check, statusStr, r.Impact)
	}

	// Calculate widths dynamically by using table.Width if available.
	// `table` package automatically manages column widths depending on contents,
	// but can optionally limit width if it's too large by letting the parent container clip or applying max width.
	// Since lipgloss tables handle layout beautifully, we can just return it.
	return tbl.Render()
}

func renderStatusBadge(status string) string {
	var color lipgloss.AdaptiveColor
	var icon string

	switch strings.ToLower(status) {
	case "success":
		color = lipgloss.AdaptiveColor{Light: "#43A047", Dark: "#1B5E20"} // pill color
		icon = "✔"
	case "warning":
		color = lipgloss.AdaptiveColor{Light: "#FB8C00", Dark: "#E65100"}
		icon = "⚠"
	case "failed":
		color = lipgloss.AdaptiveColor{Light: "#E53935", Dark: "#B71C1C"}
		icon = "✘"
	default:
		return status
	}

	// The image shows a dark pill with colored border + text/icon
	style := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Foreground(color)

	return style.Render(fmt.Sprintf("%s %s", icon, status))
}

func renderSuggestions(suggestions []string, width int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(mutedColor).
		Italic(true).
		MarginBottom(1)

	title := titleStyle.Render("💭 You might want to ask")

	chipStyle := lipgloss.NewStyle().
		Foreground(userMsgAccent).
		Background(lipgloss.AdaptiveColor{Light: "#F0F0F0", Dark: "#2A2A2A"}).
		Padding(0, 1).
		MarginRight(1).
		MarginBottom(1)

	var chips []string
	currentLine := ""
	currentLineWidth := 0

	for _, s := range suggestions {
		chip := chipStyle.Render(s)
		chipW := lipgloss.Width(chip)

		if currentLineWidth+chipW > width && currentLine != "" {
			chips = append(chips, currentLine)
			currentLine = chip
			currentLineWidth = chipW
		} else {
			if currentLine == "" {
				currentLine = chip
			} else {
				currentLine = lipgloss.JoinHorizontal(lipgloss.Top, currentLine, chip)
			}
			currentLineWidth += chipW
		}
	}
	if currentLine != "" {
		chips = append(chips, currentLine)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, lipgloss.JoinVertical(lipgloss.Left, chips...))
}

// ── Empty state ─────────────────────────────────────────────────────

func renderEmptyState(width, height int, logo string, suggestions []string) string {
	title := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}).
		Bold(true).
		Render("DuckOps AI Agent")

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}).
		Render("Type a message to get started, or press / for commands")

	hints := []string{
		"  /help          Show all available commands",
		"  /status        Show workspace status",
		"  /scan          Manage security scans",
		"  /clear         Clear the screen",
	}

	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#444444", Dark: "#666666"})
	var hintLines []string
	for _, h := range hints {
		// Use a fixed width for each line to ensure they are consistent, 
		// but JoinVertical(lipgloss.Left) on the lines is the key fix.
		hintLines = append(hintLines, hintStyle.Render(h))
	}
	hintBlock := lipgloss.JoinVertical(lipgloss.Left, hintLines...)

	if len(suggestions) == 0 {
		suggestions = []string{
			"Show workspace status",
			"How do I manage security scans?",
			"Explain DuckOps features",
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		logo,
		"",
		title,
		subtitle,
		"",
		hintBlock,
		"",
		"",
		renderSuggestions(suggestions, width-20),
	)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}
