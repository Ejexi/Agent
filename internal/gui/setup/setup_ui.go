package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/SecDuckOps/agent/internal/config"
)

// SetupResults holds the gathered configuration.
type SetupResults struct {
	ProfileName string
	Provider    string
	APIKey      string
	BaseURL     string
	Telemetry   bool
}

type state int

const (
	stateProfileName state = iota
	stateProviderSelect
	stateAPIKey
	stateTelemetry
	stateConfirm
)

type model struct {
	state       state
	profileName textinput.Model
	provider    int
	apiKey      textinput.Model
	telemetry   bool
	results     *SetupResults
	err         error
	quitting    bool
	width       int
	height      int
}

var (
	providers = []string{"openai", "anthropic", "google", "openrouter", "deepseek"}
	
	// Styles
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ffd5")).
			Bold(true).
			Padding(1, 2)
	
	stepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)
	
	activeStepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffd700")).
			Bold(true).
			Underline(true)
	
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00ffd5")).
			Padding(1, 2).
			MarginTop(1)
	
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffd700")).
			Bold(true)
	
	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			MarginTop(1)
)

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "default"
	ti.Focus()
	ti.CharLimit = 32
	ti.Width = 20

	ak := textinput.New()
	ak.Placeholder = "sk-..."
	ak.EchoMode = textinput.EchoPassword
	ak.EchoCharacter = '•'
	ak.CharLimit = 128
	ak.Width = 40

	return model{
		state:       stateProfileName,
		profileName: ti,
		apiKey:      ak,
		provider:    0,
		telemetry:   false,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		
		case tea.KeyEnter:
			switch m.state {
			case stateProfileName:
				if m.profileName.Value() == "" {
					m.profileName.SetValue("default")
				}
				m.state = stateProviderSelect
			case stateProviderSelect:
				m.state = stateAPIKey
				m.apiKey.Focus()
			case stateAPIKey:
				m.state = stateTelemetry
			case stateTelemetry:
				m.state = stateConfirm
			case stateConfirm:
				m.results = &SetupResults{
					ProfileName: m.profileName.Value(),
					Provider:    providers[m.provider],
					APIKey:      m.apiKey.Value(),
					Telemetry:   m.telemetry,
				}
				return m, tea.Quit
			}
		
		case tea.KeyUp, tea.KeyDown:
			if m.state == stateProviderSelect {
				if msg.Type == tea.KeyUp {
					m.provider--
				} else {
					m.provider++
				}
				if m.provider < 0 {
					m.provider = len(providers) - 1
				}
				if m.provider >= len(providers) {
					m.provider = 0
				}
			} else if m.state == stateTelemetry {
				m.telemetry = !m.telemetry
			}
		}
	
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	// Update active input
	if m.state == stateProfileName {
		m.profileName, cmd = m.profileName.Update(msg)
	} else if m.state == stateAPIKey {
		m.apiKey, cmd = m.apiKey.Update(msg)
	}

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString(titleStyle.Render("🦆 DUCKOPS INTERACTIVE SETUP"))
	s.WriteString("\n\n")

	// Prender STEPS
	steps := []string{"Profile", "Provider", "API Key", "Telemetry", "Confirm"}
	for i, name := range steps {
		style := stepStyle
		if int(m.state) == i {
			style = activeStepStyle
		}
		s.WriteString(style.Render(fmt.Sprintf("Step %d: %s", i+1, name)))
		if i < len(steps)-1 {
			s.WriteString("  |  ")
		}
	}
	s.WriteString("\n\n")

	switch m.state {
	case stateProfileName:
		s.WriteString("Enter profile name:\n")
		s.WriteString(m.profileName.View())
	
	case stateProviderSelect:
		s.WriteString("Choose your AI provider:\n\n")
		for i, p := range providers {
			cursor := " "
			name := p
			if i == m.provider {
				cursor = "⮕"
				name = selectedStyle.Render(p)
			}
			s.WriteString(fmt.Sprintf("  %s %s\n", cursor, name))
		}
	
	case stateAPIKey:
		s.WriteString(fmt.Sprintf("Enter API Key for %s:\n", providers[m.provider]))
		s.WriteString(m.apiKey.View())
		s.WriteString("\n\n(This will be saved as an environment variable reference if possible)")
	
	case stateTelemetry:
		s.WriteString(boxStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffd5")).Bold(true).Render("Anonymous Telemetry"),
				"We collect anonymous telemetry to improve DuckOps.",
				"No prompts, code, or personal data is collected.",
				"Opt-out by selecting 'No' below.",
			),
		))
		s.WriteString("\n\nAllow telemetry?\n")
		yes := " [ ] Yes "
		no := " [ ] No "
		if m.telemetry {
			yes = " [X] Yes "
			no = " [ ] No "
		} else {
			yes = " [ ] Yes "
			no = " [X] No "
		}
		
		if m.telemetry {
			s.WriteString(selectedStyle.Render(yes) + no)
		} else {
			s.WriteString(yes + selectedStyle.Render(no))
		}
		s.WriteString("\n\n(Use ↑/↓ to toggle)")

	case stateConfirm:
		s.WriteString("Summary:\n")
		s.WriteString(fmt.Sprintf("  Profile:   %s\n", m.profileName.Value()))
		s.WriteString(fmt.Sprintf("  Provider:  %s\n", providers[m.provider]))
		s.WriteString(fmt.Sprintf("  Telemetry: %v\n", m.telemetry))
		s.WriteString("\nPress Enter to save configuration...")
	}

	s.WriteString(hintStyle.Render("\n\n[enter] Next  |  [esc] Cancel"))

	return s.String()
}

// Run launches the TUI.
func Run(cfg *config.DuckOpsConfig) (*SetupResults, error) {
	p := tea.NewProgram(initialModel())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m := finalModel.(model)
	if m.quitting {
		return nil, nil
	}

	return m.results, nil
}
