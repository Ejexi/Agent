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
)

// Pre-built styles that are purely cosmetic (no dynamic width/height).
// Building these once avoids repeated allocations in hot render paths.
var (
	spTitleStyle = lipgloss.NewStyle().Foreground(sidePanelTitle).Bold(true)
	spSepStyle   = lipgloss.NewStyle().Foreground(sidePanelBorder)
	spDirStyle   = lipgloss.NewStyle().Foreground(sidePanelDir)
	spFileStyle  = lipgloss.NewStyle().Foreground(sidePanelFile)
	spMutedStyle = lipgloss.NewStyle().Foreground(sidePanelMuted)
	spLabelStyle = lipgloss.NewStyle().Foreground(sidePanelMuted)
	spValueStyle = lipgloss.NewStyle().Foreground(sidePanelText)
	spRootStyle  = lipgloss.NewStyle().Foreground(sidePanelDir).Bold(true)
)

// skipDirs is the set of directory/file names to ignore during tree traversal.
// Defined once at package level so collectTree doesn't allocate a new map on
// every call (including recursive ones).
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".duckops": true, "__pycache__": true, ".idea": true,
	"dist": true, "build": true, ".cache": true,
}

// RenderSidePanel renders the side panel with agent info + file tree.
func RenderSidePanel(height int, modelName string, promptTokens, completionTokens int) string {
	innerW := SidePanelWidth - 3
	sep := spSepStyle.Width(innerW).Render(strings.Repeat("─", innerW))
	titleW := spTitleStyle.Width(innerW)

	agentSection := lipgloss.JoinVertical(lipgloss.Left,
		titleW.Render("󰦆  Agent"),
		sep,
		renderSideItem("Model", truncate(modelName, innerW-8), innerW),
		renderSideItem("Mode", "TUI", innerW),
		"",
		titleW.Render("  Tokens"),
		sep,
		renderSideItem("In", formatTokens(promptTokens), innerW),
		renderSideItem("Out", formatTokens(completionTokens), innerW),
		renderSideItem("Total", formatTokens(promptTokens+completionTokens), innerW),
	)

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

func formatTokens(n int) string {
	switch {
	case n == 0:
		return "-"
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1000.0)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// renderFileTree builds a compact file tree from cwd.
func renderFileTree(width, maxLines int) string {
	if maxLines < 3 {
		return ""
	}

	sep := spSepStyle.Width(width).Render(strings.Repeat("─", width))

	cwd, _ := os.Getwd()
	dirName := filepath.Base(cwd)

	available := maxLines - 3 // title + sep + root line
	if available < 1 {
		available = 1
	}

	entries := collectTree(cwd, "", 0, 2, &available)

	lines := make([]string, 0, 3+len(entries))
	lines = append(lines,
		spTitleStyle.Width(width).Render("  Files"),
		sep,
		spRootStyle.Render("📁 "+truncate(dirName, width-3)),
	)
	lines = append(lines, entries...)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// collectTree recursively walks dir up to maxDepth levels, appending rendered
// lines and decrementing *budget so callers don't need a separate trim step.
func collectTree(dir, prefix string, depth, maxDepth int, budget *int) []string {
	if depth >= maxDepth || *budget <= 0 {
		return nil
	}

	rawEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Filter skipped entries first so isLast is computed on the visible set.
	filtered := rawEntries[:0]
	for _, e := range rawEntries {
		if !skipDirs[e.Name()] {
			filtered = append(filtered, e)
		}
	}

	// Sort: dirs first, then alphabetical.
	sort.Slice(filtered, func(i, j int) bool {
		iDir := filtered[i].IsDir()
		jDir := filtered[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return filtered[i].Name() < filtered[j].Name()
	})

	const maxShow = 20
	var lines []string

	for idx, e := range filtered {
		if *budget <= 0 || idx >= maxShow {
			lines = append(lines, spMutedStyle.Render(prefix+"  └── ..."))
			break
		}

		isLast := idx == len(filtered)-1
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		innerW := SidePanelWidth - 5
		name := truncate(e.Name(), innerW-len(prefix)-4)

		if e.IsDir() {
			line := spMutedStyle.Render(prefix+connector) + spDirStyle.Render("📁 "+name)
			lines = append(lines, line)
			*budget--

			children := collectTree(filepath.Join(dir, e.Name()), childPrefix, depth+1, maxDepth, budget)
			lines = append(lines, children...)
		} else {
			icon := fileIcon(e, dir)
			line := spMutedStyle.Render(prefix+connector) + spFileStyle.Render(icon+" "+name)
			lines = append(lines, line)
			*budget--
		}
	}

	return lines
}

// fileIcon returns a simple icon based on file extension or name.
func fileIcon(e fs.DirEntry, _ string) string {
	name := e.Name()

	// Check well-known names before falling back to extension.
	switch strings.ToLower(name) {
	case "dockerfile":
		return "🐳"
	case "makefile", "gnumakefile":
		return "🔧"
	}

	switch strings.ToLower(filepath.Ext(name)) {
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
	default:
		return "📄"
	}
}

func renderSideItem(label, value string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(
		spLabelStyle.Render(label+": ") + spValueStyle.Render(value),
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