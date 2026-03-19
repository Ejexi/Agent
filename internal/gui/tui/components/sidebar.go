package components

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SidePanelWidth is the fixed width of the side panel (including border).
const SidePanelWidth = 34

var (
	sidePanelBorder  = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}
	sidePanelTitle   = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9D7BFF"}
	sidePanelText    = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	sidePanelMuted   = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	sidePanelDir     = lipgloss.AdaptiveColor{Light: "#1565C0", Dark: "#64B5F6"}
	sidePanelFile    = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#81C784"}
	sidePanelSection = lipgloss.AdaptiveColor{Light: "#555555", Dark: "#AAAAAA"}
)

// RenderSidePanel renders the side panel with agent info + file tree.
func RenderSidePanel(height int, modelName string, promptTokens, completionTokens int) string {
	innerW := SidePanelWidth - 3

	formatTokens := func(n int) string {
		if n == 0 {
			return "-"
		}
		if n >= 1000 {
			return fmt.Sprintf("%.1fk", float64(n)/1000.0)
		}
		return fmt.Sprintf("%d", n)
	}

	titleStyle := lipgloss.NewStyle().Foreground(sidePanelTitle).Bold(true).Width(innerW)
	sepStyle := lipgloss.NewStyle().Foreground(sidePanelBorder).Width(innerW)
	sep := sepStyle.Render(strings.Repeat("─", innerW))

	// ── Agent Info ─────────────────────────────────────────────────
	agentSection := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("󰦆  Agent"),
		sep,
		renderSideItem("Model", truncate(modelName, innerW-8), innerW),
		renderSideItem("Mode", "TUI", innerW),
		"",
		titleStyle.Render("  Tokens"),
		sep,
		renderSideItem("In", formatTokens(promptTokens), innerW),
		renderSideItem("Out", formatTokens(completionTokens), innerW),
		renderSideItem("Total", formatTokens(promptTokens+completionTokens), innerW),
	)

	// ── File Tree ──────────────────────────────────────────────────
	fileTreeSection := renderFileTree(innerW, height-lipgloss.Height(agentSection)-5)

	content := lipgloss.JoinVertical(lipgloss.Left,
		agentSection,
		"",
		fileTreeSection,
	)

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderLeft(true).
		BorderRight(false).
		BorderTop(false).
		BorderBottom(false).
		BorderForeground(sidePanelBorder).
		Width(SidePanelWidth).
		Height(height).
		PaddingLeft(1).
		Render(content)
}

// renderFileTree builds a compact file tree from cwd.
func renderFileTree(width, maxLines int) string {
	if maxLines < 3 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Foreground(sidePanelTitle).Bold(true).Width(width)
	sepStyle := lipgloss.NewStyle().Foreground(sidePanelBorder).Width(width)
	sep := sepStyle.Render(strings.Repeat("─", width))

	cwd, _ := os.Getwd()
	dirName := filepath.Base(cwd)

	// Collect tree entries (max 2 levels deep)
	entries := collectTree(cwd, "", 0, 2)

	// Limit lines
	available := maxLines - 3 // title + sep + root
	if available < 1 {
		available = 1
	}
	if len(entries) > available {
		entries = entries[:available]
	}

	// Root line
	rootStyle := lipgloss.NewStyle().Foreground(sidePanelDir).Bold(true)
	lines := []string{
		titleStyle.Render("  Files"),
		sep,
		rootStyle.Render("📁 " + truncate(dirName, width-3)),
	}

	for _, e := range entries {
		lines = append(lines, e)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

type treeEntry struct {
	display string
	isDir   bool
}

func collectTree(dir, prefix string, depth, maxDepth int) []string {
	if depth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Sort: dirs first, then files, alphabetically
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return entries[i].Name() < entries[j].Name()
	})

	// Skip noise
	skip := map[string]bool{
		".git": true, "node_modules": true, "vendor": true,
		".duckops": true, "__pycache__": true, ".idea": true,
		"dist": true, "build": true, ".cache": true,
	}

	dirStyle := lipgloss.NewStyle().Foreground(sidePanelDir)
	fileStyle := lipgloss.NewStyle().Foreground(sidePanelFile)
	mutedStyle := lipgloss.NewStyle().Foreground(sidePanelMuted)

	var lines []string
	shown := 0
	maxShow := 20

	for i, e := range entries {
		if shown >= maxShow {
			lines = append(lines, mutedStyle.Render(prefix+"  └── ..."))
			break
		}
		if skip[e.Name()] {
			continue
		}

		isLast := i == len(entries)-1
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		innerW := SidePanelWidth - 5
		name := truncate(e.Name(), innerW-len(prefix)-4)

		var line string
		if e.IsDir() {
			line = mutedStyle.Render(prefix+connector) + dirStyle.Render("📁 "+name)
			lines = append(lines, line)
			shown++
			// Recurse one level
			if depth+1 < maxDepth {
				children := collectTree(filepath.Join(dir, e.Name()), childPrefix, depth+1, maxDepth)
				lines = append(lines, children...)
				shown += len(children)
			}
		} else {
			icon := fileIcon(e, dir)
			line = mutedStyle.Render(prefix+connector) + fileStyle.Render(icon+" "+name)
			lines = append(lines, line)
			shown++
		}
	}

	return lines
}

// fileIcon returns a simple icon based on file extension.
func fileIcon(e fs.DirEntry, dir string) string {
	name := e.Name()
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go":
		return "🐹"
	case ".py":
		return "🐍"
	case ".ts", ".tsx":
		return "📘"
	case ".js", ".jsx":
		return "📙"
	case ".md":
		return "📝"
	case ".json", ".yaml", ".yml", ".toml":
		return "⚙️"
	case ".sh", ".bash":
		return "🔧"
	case ".sql":
		return "🗄️"
	case ".tf", ".tfvars":
		return "🏗️"
	case ".dockerfile", "":
		if strings.EqualFold(name, "Dockerfile") {
			return "🐳"
		}
		return "📄"
	default:
		return "📄"
	}
}

func renderSideItem(label, value string, width int) string {
	labelStyle := lipgloss.NewStyle().Foreground(sidePanelMuted)
	valueStyle := lipgloss.NewStyle().Foreground(sidePanelText)
	return lipgloss.NewStyle().Width(width).Render(
		labelStyle.Render(label+": ") + valueStyle.Render(value),
	)
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
