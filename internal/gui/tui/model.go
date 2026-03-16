package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/engine"
	"github.com/SecDuckOps/agent/internal/gui/tui/terminal"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/skills"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Message types ──────────────────────────────────────────────────

type MessageType int

const (
	UserMsg MessageType = iota
	AgentMsg
	SystemMsg
	ErrorMsg
	LearningMsg
	ThoughtMsg
	ReflectionMsg
)

// Message represents a single chat message in the conversation.
type Message struct {
	Type      MessageType
	Content   string
	Sender       string
	Timestamp    time.Time
	TableHeaders []string
	TableData    [][]string
}

// AgentMessage is received from the backend via msgChan.
type AgentMessage struct {
	Content string
	Type    MessageType
	Usage        shared_domain.TokenUsage
	Model        string
	TableHeaders []string
	TableData    [][]string
}

// ── Popup types ────────────────────────────────────────────────────

type PopupType int

const (
	PopupNone PopupType = iota
	PopupShortcuts
	PopupConfirm
)

// ── Session Mode ───────────────────────────────────────────────────

type SessionMode int

const (
	ChatMode SessionMode = iota
	ShellDiscoveryMode
	ShellExecutionMode
	FileDiscoveryMode
)

// ── Toast ──────────────────────────────────────────────────────────

type ToastLevel int

const (
	ToastInfo ToastLevel = iota
	ToastSuccess
	ToastWarning
	ToastError
)

type Toast struct {
	Message string
	Level   ToastLevel
}

// ── Model ──────────────────────────────────────────────────────────

const (
	minTerminalWidth  = 80
	minTerminalHeight = 24
	maxUIWidth        = 140
	maxUIHeight       = 50
)

type layoutMetrics struct {
	mainW    int
	mainH    int
	contentW int
	contentH int
}

type model struct {
	// Terminal
	width  int
	height int
	caps   terminal.TerminalCapabilities

	// Layout state
	metrics layoutMetrics

	// Messages
	messages     []Message
	scroll       int
	stayAtBottom bool
	wrappedCache [][]string // cached wrapped lines per message

	// Input
	textarea textarea.Model

	// Loading
	spinner spinner.Model
	loading bool

	// Panels & Popups
	showSidePanel bool
	showShortcuts bool
	activePopup   PopupType
	toast         *Toast

	// Menu
	showMenu      bool
	menuSelection int

	// Engine
	engine       *engine.Engine
	isProcessing bool
	activeModel  string
	totalUsage   shared_domain.TokenUsage

	// Session & Events (Phase 1 Enhancements)
	appSessionManager ports.AppSessionManager
	eventBus          ports.EventBusPort
	skillRegistry     skills.Registry

	// Shell
	shellActive bool
	shell       *ShellModel

	// Discovery
	discovery          *CommandDiscovery
	discoveryResults   []CommandInfo
	discoveryIndex     int
	mode               SessionMode
	logo               string
	dynamicSuggestions []string

	// Paste Handling
	submitPending bool

	// Stream tracking
	lastStreamCh <-chan any
}

// NewModel creates an initialised model with the given terminal capabilities.
func NewModel(caps terminal.TerminalCapabilities, modelName string, appSessionManager ports.AppSessionManager, eventBus ports.EventBusPort, skillRegistry skills.Registry) model {
	// ── Textarea ────────────────────────────────────────────────────
	ta := textarea.New()
	ta.Placeholder = "Type a message or ! for commands"
	ta.Focus()
	ta.CharLimit = 4000
	ta.MaxHeight = 1
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle().Foreground(Theme.Text)
	ta.BlurredStyle.Base = lipgloss.NewStyle().Foreground(Theme.Muted)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(Theme.Muted)
	// Start with 1 line and let it automatically grow up to MaxHeight
	ta.SetHeight(1)

	// ── Spinner ─────────────────────────────────────────────────────
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(Theme.Accent)
	if caps.Unicode {
		sp.Spinner = spinner.Dot
	} else {
		sp.Spinner = spinner.Line
	}

	cwd, _ := os.Getwd()
	eng := engine.NewEngine(cwd)

	asciiLogo := `
██████╗ ██╗   ██╗██████╗ ██╗  ██╗ ██████╗ ██████╗ ███████╗
██╔══██╗██║   ██║██╔════╝██║ ██╔╝██╔═══██╗██╔══██╗██╔════╝
██║  ██║██║   ██║██║     █████╔╝ ██║   ██║██████╔╝███████╗
██║  ██║██║   ██║██║     ██╔═██╗ ██║   ██║██╔═══╝ ╚════██║
██████╔╝╚██████╔╝╚██████╗██║  ██╗╚██████╔╝██║     ███████║
╚═════╝  ╚═════╝  ╚═════╝╚═╝  ╚═╝ ╚═════╝ ╚═╝     ╚══════╝
`
	// Apply horizontal gradient
	lines := strings.Split(asciiLogo, "\n")
	var coloredLines []string
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		var coloredLine string
		chars := []rune(line)
		for i, char := range chars {
			// Calculate gradient from left to right (Dark Gray to White)
			// Start: #444444, End: #FFFFFF
			ratio := float64(i) / float64(len(chars))
			
			// Color interpolation from dark gray to white
			// To avoid being completely invisible on black terminals, we start at 0x44
			val := int(0x44 + (0xFF-0x44)*ratio)
			
			hexColor := fmt.Sprintf("#%02X%02X%02X", val, val, val)
			
			coloredLine += lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor)).Render(string(char))
		}
		coloredLines = append(coloredLines, coloredLine)
	}

	logo := lipgloss.JoinVertical(lipgloss.Left, coloredLines...) + "\n"

	return model{
		caps:               caps,
		textarea:           ta,
		spinner:            sp,
		messages:           []Message{},
		stayAtBottom:       true,
		showMenu:           false,
		engine:             eng,
		activeModel:        modelName,
		shell:              NewShellModel(80, 24), // Initial size, will be updated by WindowSizeMsg
		discovery:          NewCommandDiscovery(),
		mode:               ChatMode,
		logo:               logo,
		dynamicSuggestions: eng.GetSuggestions(context.Background()),
		appSessionManager:  appSessionManager,
		eventBus:           eventBus,
		skillRegistry:      skillRegistry,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, tea.EnableBracketedPaste)
}
