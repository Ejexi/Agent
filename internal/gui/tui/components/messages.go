package components

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// ── Markdown Rendering ─────────────────────────────────────────────

// rendererCache caches glamour renderers keyed by width to avoid
// re-parsing the theme JSON and rebuilding the renderer on every call.
var (
	rendererCache   = make(map[int]*glamour.TermRenderer)
	rendererCacheMu sync.Mutex
)

func renderMarkdown(content string, width int) string {
	rendererCacheMu.Lock()
	r, ok := rendererCache[width]
	if !ok {
		var err error
		r, err = glamour.NewTermRenderer(
			glamour.WithStylesFromJSONBytes(GetDuckOpsTheme()),
			glamour.WithWordWrap(width),
			glamour.WithEmoji(),
		)
		if err != nil {
			rendererCacheMu.Unlock()
			return wordwrap.String(content, width)
		}
		rendererCache[width] = r
	}
	rendererCacheMu.Unlock()

	out, err := r.Render(content)
	if err != nil {
		return wordwrap.String(content, width)
	}
	return strings.TrimSpace(out)
}

// ChatMessage is the component-level representation of a message.
type ChatMessage struct {
	Type         int // 0=User, 1=Agent, 2=System, 3=Error
	Content      string
	Sender       string
	Timestamp    time.Time
	TableHeaders []string
	TableData    [][]string
	Suggestions  []string
}

// ChatMessage type constants (mirror model-level enum w/o import cycle).
const (
	ChatUser       = 0
	ChatAgent      = 1
	ChatSystem     = 2
	ChatError      = 3
	ChatLearning   = 4
	ChatThought    = 5
	ChatReflection = 6
)

// ── Message type metadata ───────────────────────────────────────────

// msgMeta holds all per-type styling and label data in one place,
// eliminating the dual switch statements in renderSingleMessage.
type msgMeta struct {
	accent    lipgloss.AdaptiveColor
	indicator string // bar/bullet prefix
	header    func(sender, timeStr string) string
}

var msgMetaMap = map[int]msgMeta{
	ChatUser: {
		accent:    lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#7D56F4"},
		indicator: "│",
	},
	ChatAgent: {
		accent:    lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#34D399"},
		indicator: "│",
	},
	ChatSystem: {
		accent:    lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"},
		indicator: "│",
	},
	ChatError: {
		accent:    lipgloss.AdaptiveColor{Light: "#D32F2F", Dark: "#FF6B6B"},
		indicator: "│",
	},
	ChatLearning: {
		accent:    lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#6B7280"},
		indicator: "•",
	},
	ChatThought: {
		accent:    lipgloss.AdaptiveColor{Light: "#8B5CF6", Dark: "#A78BFA"},
		indicator: "║",
	},
	ChatReflection: {
		accent:    lipgloss.AdaptiveColor{Light: "#0D9488", Dark: "#2DD4BF"},
		indicator: "║",
	},
}

var (
	textColor  = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	mutedColor = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
)

func metaFor(msgType int) msgMeta {
	if m, ok := msgMetaMap[msgType]; ok {
		return m
	}
	return msgMeta{
		accent:    textColor,
		indicator: "│",
	}
}

// ── Rendered-line cache ─────────────────────────────────────────────

// renderedLines caches the per-message line-split result so
// RenderMessageArea and MaxScroll don't both re-render everything.
type renderedMsg struct {
	lines []string
}

func renderAllMessages(msgs []ChatMessage, contentWidth int) []renderedMsg {
	out := make([]renderedMsg, len(msgs))
	for i, msg := range msgs {
		rendered := renderSingleMessage(msg, contentWidth)
		out[i] = renderedMsg{lines: strings.Split(rendered, "\n")}
	}
	return out
}

// RenderMessageArea renders the full messages area at the given dimensions.
// `scrollOffset` is measured from the bottom (0 = at bottom).
func RenderMessageArea(msgs []ChatMessage, width, height, scrollOffset int, logo string, suggestions []string) string {
	if width < 4 {
		width = 4
	}

	contentWidth := width - 4 // 4 = left/right margins
	rendered := renderAllMessages(msgs, contentWidth)

	// Flatten all lines.
	var allLines []string
	for _, rm := range rendered {
		allLines = append(allLines, rm.lines...)
	}

	if len(allLines) == 0 {
		return renderEmptyState(width, height, logo, suggestions)
	}

	// Clamp scroll offset and compute visible window.
	totalLines := len(allLines)
	if scrollOffset > totalLines-height {
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

	// Pad with empty lines at the top to fill the viewport.
	for len(visible) < height {
		visible = append([]string{""}, visible...)
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(strings.Join(visible, "\n"))
}

// MaxScroll returns the maximum scroll offset for the given messages.
func MaxScroll(msgs []ChatMessage, width, height int) int {
	contentWidth := width - 4
	rendered := renderAllMessages(msgs, contentWidth)

	var totalLines int
	for _, rm := range rendered {
		totalLines += len(rm.lines)
	}
	if max := totalLines - height; max > 0 {
		return max
	}
	return 0
}

// ── Single message rendering ────────────────────────────────────────

func renderSingleMessage(msg ChatMessage, contentWidth int) string {
	if contentWidth < 10 {
		contentWidth = 10
	}

	meta := metaFor(msg.Type)

	// Determine per-type content widths.
	bodyWidth := contentWidth - 4 // indicator bar + padding
	if msg.Type == ChatThought || msg.Type == ChatReflection {
		bodyWidth = contentWidth - 8 // full borders + padding
	}

	var body string
	switch msg.Type {
	case ChatAgent, ChatError, ChatThought, ChatReflection:
		body = parseAndRenderMixedContent(msg.Content, bodyWidth)
	default:
		body = wordwrap.String(msg.Content, bodyWidth)
	}

	// Header line.
	senderStyle := lipgloss.NewStyle().Foreground(meta.accent).Bold(true)
	timeStyle := lipgloss.NewStyle().Foreground(mutedColor).Italic(true)
	timeStr := msg.Timestamp.Format("15:04")

	var header string
	switch msg.Type {
	case ChatLearning:
		header = fmt.Sprintf("%s", lipgloss.NewStyle().Foreground(meta.accent).Italic(true).Render("Step..."))
	case ChatThought:
		header = fmt.Sprintf("\n%s", lipgloss.NewStyle().Foreground(meta.accent).Bold(true).Render(" 🧠  DuckOps is thinking..."))
	case ChatReflection:
		header = fmt.Sprintf("\n%s", lipgloss.NewStyle().Foreground(meta.accent).Bold(true).Render(" 💡  DuckOps is reflecting..."))
	default:
		sentCue := ""
		if msg.Type == ChatUser {
			sentCue = " " + lipgloss.NewStyle().Foreground(meta.accent).Render("✓")
		}
		header = fmt.Sprintf("%s  %s%s", senderStyle.Render(msg.Sender), timeStyle.Render(timeStr), sentCue)
	}

	// Body rendering.
	isPureShell := strings.HasPrefix(msg.Content, "```shell") &&
		strings.HasSuffix(strings.TrimSpace(msg.Content), "```")

	var renderedBody string
	if isPureShell {
		clean := strings.TrimSpace(msg.Content)
		clean = strings.TrimPrefix(clean, "```shell")
		clean = strings.TrimSuffix(clean, "```")
		clean = strings.TrimSpace(clean)
		renderedBody = renderMarkdown("```\n"+clean+"\n```", bodyWidth)
	} else {
		switch msg.Type {
		case ChatLearning:
			renderedBody = lipgloss.NewStyle().
				Foreground(meta.accent).
				Italic(true).
				Render(meta.indicator + " " + body)
		case ChatThought, ChatReflection:
			renderedBody = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(meta.accent).
				Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#B0B0B0"}).
				Padding(0, 1).
				MarginTop(1).
				Render(body)
		default:
			renderedBody = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(meta.accent).
				PaddingLeft(2).
				Render(body)
		}
	}

	parts := []string{header, renderedBody}
	if len(msg.TableData) > 0 {
		parts = append(parts, "", renderTable(msg.TableHeaders, msg.TableData, contentWidth))
	}
	if len(msg.Suggestions) > 0 {
		parts = append(parts, "", renderSuggestions(msg.Suggestions, contentWidth))
	}
	// Trailing blank line for spacing between messages.
	parts = append(parts, "")

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ── Table rendering ─────────────────────────────────────────────────

func renderTable(headers []string, rows [][]string, width int) string {
	if len(rows) == 0 || len(headers) == 0 {
		return ""
	}

	numCols := len(headers)
	// 3 chars per column for border+padding, 2 for outer border.
	availWidth := width - (numCols*3 + 2)
	if availWidth < 10*numCols {
		availWidth = 10 * numCols
	}

	colWidths := make([]int, numCols)
	maxPerCol := availWidth / 3

	for c := 0; c < numCols-1; c++ {
		maxW := len(headers[c])
		for _, r := range rows {
			if c < len(r) {
				if l := lipgloss.Width(r[c]); l > maxW {
					maxW = l
				}
			}
		}
		if maxW > maxPerCol {
			maxW = maxPerCol
		}
		colWidths[c] = maxW
	}

	// Last column gets whatever is left; guarantee a minimum of 10.
	used := 0
	for c := 0; c < numCols-1; c++ {
		used += colWidths[c]
	}
	last := availWidth - used
	if last < 10 {
		last = 10
	}
	colWidths[numCols-1] = last

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fc0404ff")).Bold(true).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))

	isStatusTable := len(headers) == 3 && headers[1] == "Status"

	// Pre-allocate with a known capacity.
	blocks := make([]string, 0, 2+len(rows))

	headerCells := make([]string, numCols)
	for c, h := range headers {
		headerCells[c] = headerStyle.Width(colWidths[c] + 2).Render(h)
	}
	blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Left, headerCells...))

	sepCells := make([]string, numCols)
	for c := range sepCells {
		sepCells[c] = borderStyle.Render(strings.Repeat("─", colWidths[c]+2))
	}
	blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Left, sepCells...))

	cells := make([]string, numCols)
	for _, r := range rows {
		for c := 0; c < numCols && c < len(r); c++ {
			content := wordwrap.String(r[c], colWidths[c])
			if isStatusTable && c == 1 {
				content = renderStatusBadge(r[c])
			}
			cells[c] = cellStyle.Width(colWidths[c] + 2).Render(content)
		}
		blocks = append(blocks, lipgloss.JoinHorizontal(lipgloss.Top, cells[:min(numCols, len(r))]...))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#555555")).
		Render(lipgloss.JoinVertical(lipgloss.Left, blocks...))
}

func renderStatusBadge(status string) string {
	var color lipgloss.AdaptiveColor
	var icon string

	switch strings.ToLower(status) {
	case "success":
		color = lipgloss.AdaptiveColor{Light: "#43A047", Dark: "#1B5E20"}
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

	return lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Foreground(color).
		Render(fmt.Sprintf("%s %s", icon, status))
}

func renderSuggestions(suggestions []string, width int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(mutedColor).
		Italic(true).
		MarginBottom(1)

	chipStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#2A2A2A"}).
		Padding(0, 1).
		MarginRight(1).
		MarginBottom(1)

	// Render chips once, measure once.
	type chip struct {
		rendered string
		width    int
	}
	chips := make([]chip, len(suggestions))
	for i, s := range suggestions {
		r := chipStyle.Render(s)
		chips[i] = chip{r, lipgloss.Width(r)}
	}

	var lines []string
	var currentParts []string
	currentWidth := 0

	for _, c := range chips {
		if currentWidth+c.width > width && len(currentParts) > 0 {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, currentParts...))
			currentParts = currentParts[:0]
			currentWidth = 0
		}
		currentParts = append(currentParts, c.rendered)
		currentWidth += c.width
	}
	if len(currentParts) > 0 {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, currentParts...))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("💭 You might want to ask"),
		lipgloss.JoinVertical(lipgloss.Left, lines...),
	)
}

// ── Empty state ─────────────────────────────────────────────────────

func renderEmptyState(width, height int, logo string, suggestions []string) string {
	innerW := width - 10
	if innerW > 90 {
		innerW = 90
	}
	if innerW < 30 {
		innerW = 30
	}

	textStyle := lipgloss.NewStyle().Foreground(textColor)
	mutedStyle := lipgloss.NewStyle().Foreground(mutedColor)

	modesBlock := lipgloss.JoinVertical(lipgloss.Left,
		textStyle.Render("Modes:"),
		mutedStyle.Render("  • Type '/' to browse available commands"),
		mutedStyle.Render("  • Type '!' to run a command directly"),
		mutedStyle.Render("  • Type '@' to search files and directories"),
		mutedStyle.Render("  • Or just start typing to ask the AI"),
	)

	sepLine := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#DDDDDD", Dark: "#3F3F3F"}).
		Render(strings.Repeat("─", innerW))

	leftItems := []struct{ cmd, desc string }{
		{"/help", "Show all available commands"},
		{"/status", "Show workspace status"},
		{"/projects", "List accessible projects"},
		{"/scan", "Manage security scans"},
		{"/vuln", "View vulnerabilities"},
		{"/clear", "Clear the screen"},
		{"/logout", "Sign out"},
	}
	leftLines := make([]string, 0, len(leftItems)+2)
	leftLines = append(leftLines, textStyle.Render("Type '/' to browse commands:"), "")
	for _, item := range leftItems {
		leftLines = append(leftLines, lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Foreground(textColor).Width(15).Render(item.cmd),
			lipgloss.NewStyle().Foreground(mutedColor).Render(item.desc),
		))
	}
	leftBlock := lipgloss.JoinVertical(lipgloss.Left, leftLines...)

	rightItems := []struct{ cmd, desc string }{
		{"!help", ""}, {"!status", ""}, {"!projects", ""}, {"!pipelines", ""},
		{"!scan", "backend-api-service"}, {"!vuln", "critical"}, {"@main.go", "search for files"},
	}
	rightLines := make([]string, 0, len(rightItems)+2)
	rightLines = append(rightLines, textStyle.Render("Type '!' or '@' for tools:"), "")
	for _, item := range rightItems {
		rightLines = append(rightLines, lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Foreground(textColor).Width(15).Render(item.cmd),
			lipgloss.NewStyle().Foreground(mutedColor).Render(item.desc),
		))
	}
	rightBlock := lipgloss.JoinVertical(lipgloss.Left, rightLines...)

	var cols string
	if innerW < 75 {
		cols = lipgloss.JoinVertical(lipgloss.Left, leftBlock, "", rightBlock)
	} else {
		cols = lipgloss.JoinHorizontal(lipgloss.Top,
			leftBlock,
			lipgloss.NewStyle().Width(8).Render(""),
			rightBlock,
		)
	}

	elements := []string{lipgloss.PlaceHorizontal(innerW, lipgloss.Center, logo)}

	modesH := lipgloss.Height(modesBlock)
	colsH := lipgloss.Height(cols)
	suggsBlock := renderSuggestions(suggestions, innerW)

	if height > 16 {
		elements = append(elements, "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, modesBlock))
	}
	if height > modesH+colsH+22 {
		elements = append(elements, "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, sepLine), "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, cols))
	}
	if height > modesH+18 {
		elements = append(elements, "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, sepLine), "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, suggsBlock))
	} else if height > 14 {
		elements = append(elements, "", lipgloss.PlaceHorizontal(innerW, lipgloss.Center, suggsBlock))
	}

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Width(innerW).Render(
			lipgloss.JoinVertical(lipgloss.Center, elements...),
		),
	)
}

// ── Mixed content parser ────────────────────────────────────────────

func parseAndRenderMixedContent(content string, width int) string {
	lines := strings.Split(content, "\n")
	blocks := make([]string, 0, 4)

	var textBuf strings.Builder
	var tableHeaders []string
	var tableRows [][]string
	inTable := false
	inCodeBlock := false

	flushText := func() {
		if textBuf.Len() > 0 {
			blocks = append(blocks, renderMarkdown(textBuf.String(), width))
			textBuf.Reset()
		}
	}
	flushTable := func() {
		if len(tableHeaders) > 0 {
			blocks = append(blocks, renderTable(tableHeaders, tableRows, width))
			tableHeaders = nil
			tableRows = nil
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}

		if !inTable {
			if !inCodeBlock && strings.Count(trimmed, "|") >= 1 && i+1 < len(lines) {
				nextTrimmed := strings.TrimSpace(lines[i+1])
				if isMarkdownTableSeparator(nextTrimmed) {
					flushText()
					inTable = true
					tableHeaders = parseMarkdownTableRow(trimmed)
					i++ // skip separator row
					continue
				}
			}
			textBuf.WriteString(line)
			textBuf.WriteByte('\n')
		} else {
			if !inCodeBlock && strings.Count(trimmed, "|") >= 1 && trimmed != "" {
				tableRows = append(tableRows, parseMarkdownTableRow(trimmed))
			} else {
				flushTable()
				inTable = false
				textBuf.WriteString(line)
				textBuf.WriteByte('\n')
			}
		}
	}

	if inTable {
		flushTable()
	} else {
		flushText()
	}

	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

func isMarkdownTableSeparator(line string) bool {
	if !strings.Contains(line, "|") || !strings.Contains(line, "-") {
		return false
	}
	for _, ch := range line {
		if ch != ' ' && ch != '|' && ch != '-' && ch != ':' {
			return false
		}
	}
	return true
}

func parseMarkdownTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}