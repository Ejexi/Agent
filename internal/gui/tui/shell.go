package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
)

// ShellOutputMsg carries raw bytes from the PTY.
type ShellOutputMsg []byte

// ShellExitMsg carries the exit status.
type ShellExitMsg struct {
	Err      error
	ExitCode int
}

// ToggleShellMsg is sent to switch between Chat and Shell.
type ToggleShellMsg struct{}

// ShellModel handles the interactive PTY terminal.
type ShellModel struct {
	reader io.ReadCloser
	writer io.WriteCloser
	cmd    *exec.Cmd

	viewport viewport.Model
	width    int
	height   int

	buffer      []byte // Internal ring-buffer for output
	active      bool
	commandName string // To detect interactive apps
	focused     bool

	mu sync.Mutex
}

func NewShellModel(width, height int) *ShellModel {
	vp := viewport.New(width, height)

	return &ShellModel{
		viewport: vp,
		width:    width,
		height:   height,
	}
}

func (m *ShellModel) Init() tea.Cmd {
	return nil // Don't spawn automatically
}

func (m *ShellModel) RunCommand(command string, args []string) tea.Cmd {
	m.active = true
	m.commandName = command
	m.buffer = nil
	m.viewport.SetContent("Starting...")

	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		// Detect available shell: PowerShell Core (pwsh) -> PowerShell (powershell) -> CMD
		shell := "cmd.exe"
		var shellArgs []string

		if _, err := exec.LookPath("pwsh"); err == nil {
			shell = "pwsh"
			shellArgs = []string{"-NoProfile", "-Command", command + " " + strings.Join(args, " ")}
		} else if _, err := exec.LookPath("powershell"); err == nil {
			shell = "powershell"
			shellArgs = []string{"-NoProfile", "-Command", command + " " + strings.Join(args, " ")}
		} else {
			shellArgs = append([]string{"/C", command}, args...)
		}

		c = exec.Command(shell, shellArgs...)
	} else {
		// Linux/macOS: Try bash then sh
		shell := "sh"
		if _, err := exec.LookPath("bash"); err == nil {
			shell = "bash"
		}
		fullCmd := command + " " + strings.Join(args, " ")
		c = exec.Command(shell, "-c", fullCmd)
	}

	c.Env = append(os.Environ(), "TERM=xterm-256color", "PAGER=cat")

	if runtime.GOOS == "windows" {
		// pty.Start doesn't support Windows. Use pipes instead.
		pr, pw, _ := os.Pipe() // Output: Cmd -> TUI
		ir, iw, _ := os.Pipe() // Input:  TUI -> Cmd

		c.Stdout = pw
		c.Stderr = pw
		c.Stdin = ir

		if err := c.Start(); err != nil {
			_ = pw.Close()
			_ = pr.Close()
			_ = iw.Close()
			_ = ir.Close()
			return func() tea.Msg { return ShellExitMsg{Err: err, ExitCode: -1} }
		}

		m.reader = pr
		m.writer = iw
		m.cmd = c

		// We need to close the write-end once the command exits,
		// otherwise our reader will block forever.
		go func() {
			_ = c.Wait()
			_ = pw.Close()
		}()
	} else {
		f, err := pty.Start(c)
		if err != nil {
			return func() tea.Msg { return ShellExitMsg{Err: err, ExitCode: -1} }
		}

		m.reader = f
		m.writer = f
		m.cmd = c

		_ = pty.Setsize(f, &pty.Winsize{
			Cols: uint16(m.width),
			Rows: uint16(m.height),
		})
	}

	return tea.Batch(
		m.readOutput(),
		m.waitForExit(),
	)
}

func (m *ShellModel) readOutput() tea.Cmd {
	return func() tea.Msg {
		if m.reader == nil {
			return nil
		}
		buf := make([]byte, 4096)
		n, err := m.reader.Read(buf)
		if n > 0 {
			return ShellOutputMsg(buf[:n])
		}
		if err != nil {
			return nil
		}
		return nil
	}
}

func (m *ShellModel) waitForExit() tea.Cmd {
	return func() tea.Msg {
		err := m.cmd.Wait()
		if err != nil && strings.Contains(err.Error(), "already called Wait") {
			err = nil
		}

		code := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else if err == nil && m.cmd.ProcessState != nil {
			code = m.cmd.ProcessState.ExitCode()
		}
		return ShellExitMsg{Err: err, ExitCode: code}
	}
}

func (m *ShellModel) Update(msg tea.Msg) (*ShellModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case ShellOutputMsg:
		m.buffer = append(m.buffer, msg...)
		if len(m.buffer) > 50000 {
			m.buffer = m.buffer[len(m.buffer)-50000:]
		}
		// Keep ANSI colors intact — the viewport can render them
		m.viewport.SetContent(string(m.buffer))
		m.viewport.GotoBottom()
		return m, m.readOutput()

	case ShellExitMsg:
		m.active = false
		if m.reader != nil {
			_ = m.reader.Close()
			m.reader = nil
		}
		if m.writer != nil {
			_ = m.writer.Close()
			m.writer = nil
		}

		interactive := map[string]bool{"vim": true, "nano": true, "top": true, "htop": true, "ssh": true}
		if msg.ExitCode == 0 && !interactive[m.commandName] {
			return m, tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
				return ToggleShellMsg{}
			})
		}

		if msg.ExitCode != 0 {
			m.viewport.SetContent(m.viewport.View() + fmt.Sprintf("\n\n[Process exited with code %d]", msg.ExitCode))
		}
		return m, nil

	case tea.KeyMsg:
		if m.active && m.writer != nil {
			_, _ = m.writer.Write([]byte(msg.String()))
			if msg.Type == tea.KeyEnter {
				_, _ = m.writer.Write([]byte("\r\n"))
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
		if m.writer != nil && runtime.GOOS != "windows" {
			if f, ok := m.writer.(*os.File); ok {
				_ = pty.Setsize(f, &pty.Winsize{
					Cols: uint16(m.width),
					Rows: uint16(m.height),
				})
			}
		}
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *ShellModel) View() string {
	return m.viewport.View()
}
