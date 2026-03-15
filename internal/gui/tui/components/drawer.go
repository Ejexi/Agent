package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	drawerBorder = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}
	drawerTitle  = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#9B7DFF"}
)

func CommandDrawer(results []interface{}, selectedIndex int, width int, title string) string {
	if len(results) == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(drawerTitle).
		Bold(true).
		Padding(0, 1)

	header := titleStyle.Render(title)

	var rows []string
	for i, res := range results {
		// We use interface{} here to avoid circular imports if needed, 
		// but we expect CommandInfo or FileInfo
		name := ""
		info := ""
		
		if m, ok := res.(map[string]string); ok {
			name = m["name"]
			info = m["info"]
		} else {
			name = fmt.Sprintf("%v", res)
		}

		style := lipgloss.NewStyle().Padding(0, 1)
		if i == selectedIndex {
			style = style.
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#7D56F4")).
				Bold(true)
		} else {
			style = style.Foreground(lipgloss.Color("#EEEEEE"))
		}

		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Italic(true)
		if i == selectedIndex {
			infoStyle = infoStyle.Foreground(lipgloss.Color("#DDDDDD"))
		}

		row := style.Render(fmt.Sprintf("%-30s", name)) + " " + infoStyle.Render(info)
		rows = append(rows, row)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		strings.Join(rows, "\n"),
	)

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(drawerBorder).
		Width(width - 4).
		Render(content)
}
