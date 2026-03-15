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
	ChatUser     = 0
	ChatAgent    = 1
	ChatSystem   = 2
	ChatError    = 3
	ChatLearning = 4
)

// ── Adaptive message colours ────────────────────────────────────────

var (
	userMsgAccent = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#555555"}
	agentAccent   = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#34D399"}
	systemAccent  = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	errorAccent    = lipgloss.AdaptiveColor{Light: "#D32F2F", Dark: "#FF6B6B"}
	learningAccent = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#6B7280"}
	textColor      = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	mutedColor     = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
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
	case ChatLearning:
		accent = learningAccent
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

	var header string
	if msg.Type == ChatLearning {
		// Learning messages have a more compact header
		header = fmt.Sprintf("%s", lipgloss.NewStyle().Foreground(accent).Italic(true).Render("Step..."))
	} else {
		header = fmt.Sprintf("%s  %s%s", senderStyle.Render(msg.Sender), timeStyle.Render(timeStr), sentCue)
	}

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
		indicator := "│"
		if msg.Type == ChatLearning {
			indicator = "•"
		}
		indicatorStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, msg.Type != ChatLearning).
			BorderForeground(accent).
			PaddingLeft(2)
		
		if msg.Type == ChatLearning {
			renderedBody = lipgloss.NewStyle().Foreground(accent).Italic(true).Render(indicator + " " + body)
		} else {
			renderedBody = indicatorStyle.Render(body)
		}
	}

	res := lipgloss.JoinVertical(lipgloss.Left, header, renderedBody)

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

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#555555"}).Bold(true)
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
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#2A2A2A"}).
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
	// Make it responsive but with a reasonable max width
	innerW := width - 10
	if innerW > 90 {
		innerW = 90
	}
	if innerW < 30 {
		innerW = 30
	}

	// Provide styles
	textStyle := lipgloss.NewStyle().Foreground(textColor)
	mutedStyle := lipgloss.NewStyle().Foreground(mutedColor)

	modesTitle := textStyle.Render("Modes:")
	modesList := lipgloss.JoinVertical(lipgloss.Left,
		mutedStyle.Render("  • Type '/' to browse available commands"),
		mutedStyle.Render("  • Type '!' to run a command directly"),
		mutedStyle.Render("  • Type '@' to search files and directories"),
		mutedStyle.Render("  • Or just start typing to ask the AI"),
	)
	modesBlock := lipgloss.JoinVertical(lipgloss.Left, modesTitle, modesList)

	sepLine := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#DDDDDD", Dark: "#3F3F3F"}).Render(strings.Repeat("─", innerW))

	// Left Column
	colLeftTitle := textStyle.Render("Type '/' to browse commands:")
	leftItems := []struct{ cmd, desc string }{
		{"/help", "Show all available commands"},
		{"/status", "Show workspace status"},
		{"/projects", "List accessible projects"},
		{"/scan", "Manage security scans"},
		{"/vuln", "View vulnerabilities"},
		{"/clear", "Clear the screen"},
		{"/logout", "Sign out"},
	}
	var leftLines []string
	leftLines = append(leftLines, colLeftTitle, "")
	for _, item := range leftItems {
		leftLines = append(leftLines, lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Foreground(textColor).Width(15).Render(item.cmd),
			lipgloss.NewStyle().Foreground(mutedColor).Render(item.desc),
		))
	}
	leftBlock := lipgloss.JoinVertical(lipgloss.Left, leftLines...)

	// Right Column
	colRightTitle := textStyle.Render("Type '!' or '@' for tools:")
	rightItems := []struct{ cmd, desc string }{
		{"!help", ""},
		{"!status", ""},
		{"!projects", ""},
		{"!pipelines", ""},
		{"!scan", "backend-api-service"},
		{"!vuln", "critical"},
		{"@main.go", "search for files"},
	}
	var rightLines []string
	rightLines = append(rightLines, colRightTitle, "")
	for _, item := range rightItems {
		rightLines = append(rightLines, lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Foreground(textColor).Width(15).Render(item.cmd),
			lipgloss.NewStyle().Foreground(mutedColor).Render(item.desc),
		))
	}
	rightBlock := lipgloss.JoinVertical(lipgloss.Left, rightLines...)

	var cols string
	if innerW < 75 {
		cols = lipgloss.JoinVertical(lipgloss.Left,
			leftBlock,
			"",
			rightBlock,
		)
	} else {
		cols = lipgloss.JoinHorizontal(lipgloss.Top,
			leftBlock,
			lipgloss.NewStyle().Width(8).Render(""), // 8 spaces between columns
			rightBlock,
		)
	}

	if len(suggestions) == 0 {
		suggestions = []string{
			"Show workspace status",
			"How do I manage security scans?",
			"Explain DuckOps features",
		}
	}

	var elements []string
	// 1. Logo is the highest priority
	elements = append(elements, lipgloss.PlaceHorizontal(innerW, lipgloss.Center, logo))

	// 2. Add elements dynamically based on available terminal height
	if height > 16 {
		elements = append(elements, "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, modesBlock))
	}

	// Determine if cols block will fit
	colsHeight := lipgloss.Height(cols)
	if height > lipgloss.Height(modesBlock)+colsHeight+22 {
		elements = append(elements, "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, sepLine), "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, cols))
	}

	if height > lipgloss.Height(modesBlock)+18 {
		elements = append(elements, "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, sepLine), "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, renderSuggestions(suggestions, innerW)))
	} else if height > 14 {
		// Just squeeze in suggestions
		elements = append(elements, "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, renderSuggestions(suggestions, innerW)))
	}

	// Wrapper for center-aligned content
	textBlocksWrapper := lipgloss.NewStyle().Width(innerW).Render(
		lipgloss.JoinVertical(lipgloss.Center, elements...),
	)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, textBlocksWrapper)
}
