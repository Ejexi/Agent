package tui

import (
	"fmt"

	"github.com/SecDuckOps/agent/internal/gui/tui/terminal"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/skills"
	tea "github.com/charmbracelet/bubbletea"
)

// Run launches the interactive DuckOps TUI.
func Run(k *kernel.Kernel, modelName string, appSessionManager ports.AppSessionManager, sessionManager ports.SessionManager, eventBus ports.EventBusPort, skillRegistry skills.Registry) error {
	// 1. Detect terminal capabilities
	caps := terminal.DetectTerminal()

	// 2. Create the model
	m := NewModel(caps, modelName, appSessionManager, sessionManager, eventBus, skillRegistry)

	// 3. Inject the kernel into the engine
	m.engine.SetKernel(k)

	// 4. Start the program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}
