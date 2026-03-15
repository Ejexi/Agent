package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// MenuItem defines a slash command in the menu.
type MenuItem struct {
	Command     string
	Description string
}

// MenuItems is the list of available commands.
var MenuItems = []MenuItem{
	{Command: "/help", Description: "Show all available commands"},
	{Command: "/status", Description: "Show workspace status"},
	{Command: "/projects", Description: "List accessible projects"},
	{Command: "/scan", Description: "Manage security scans"},
	{Command: "/vuln", Description: "View vulnerabilities"},
	{Command: "/clear", Description: "Clear the screen"},
	{Command: "/logout", Description: "Sign out"},
}

var (
	menuBorder   = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}
	menuText     = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#e0e0e0"}
	menuMuted    = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	menuSelected = lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#555555"}
	menuWidth    = 46
)

// GetFilteredMenuItems returns menu items that start with the given filter.
func GetFilteredMenuItems(filter string) []MenuItem {
	if filter == "" || filter == "/" {
		return MenuItems
	}
	var res []MenuItem
	for _, item := range MenuItems {
		// Use strings.HasPrefix
		if len(item.Command) >= len(filter) && item.Command[:len(filter)] == filter {
			res = append(res, item)
		}
	}
	return res
}

// Menu renders the command menu with the selected item highlighted.
func Menu(items []MenuItem, selectedIndex int) string {
	if len(items) == 0 {
		return ""
	}
	if selectedIndex < 0 {
		selectedIndex = 0
	}
	if selectedIndex >= len(items) {
		selectedIndex = len(items) - 1
	}

	contentWidth := menuWidth - 4

	// Styles
	itemStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Width(contentWidth)

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Width(contentWidth)

	cmdStyle := lipgloss.NewStyle().Foreground(menuSelected).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(menuMuted)
	normalCmdStyle := lipgloss.NewStyle().Foreground(menuText)

	var linesRaw []string

	// Determine max command width for alignment
	maxCmdWidth := 0
	for _, item := range items {
		if len(item.Command) > maxCmdWidth {
			maxCmdWidth = len(item.Command)
		}
	}

	for i, item := range items {
		padLen := maxCmdWidth - len(item.Command) + 2
		pad := strings.Repeat(" ", padLen)
		
		var line string
		if i == selectedIndex {
			// Highlighted item
			cmdPart := cmdStyle.Render(item.Command)
			descPart := descStyle.Render(item.Description)
			line = selectedStyle.Render(fmt.Sprintf("▸ %s%s%s", cmdPart, pad, descPart))
		} else {
			// Normal item
			cmdPart := normalCmdStyle.Render(item.Command)
			descPart := descStyle.Render(item.Description)
			line = itemStyle.Render(fmt.Sprintf("  %s%s%s", cmdPart, pad, descPart))
		}
		linesRaw = append(linesRaw, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, linesRaw...)

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(menuBorder).
		Padding(0, 1). // 1 cell padding left/right
		Width(menuWidth).
		Render(content)
}
