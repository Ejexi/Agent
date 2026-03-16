package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/creack/pty"
	"github.com/google/uuid"
)

// OSExecAdapter implements the CommandExecutorPort using Go's native os/exec.
// It executes commands on the Host OS securely as the final stage of the Pipeline.
type OSExecAdapter struct {
	logger   shared_ports.Logger
	sessions map[string]*shellSession
	mu       sync.RWMutex
}

type shellSession struct {
	domain.ShellSession
	cmd    *exec.Cmd
	pty    *os.File
	cancel context.CancelFunc
	subs   []chan domain.ShellOutput
	mu     sync.Mutex
}

// NewOSExecAdapter creates a new executor that runs tasks on the native host.
func NewOSExecAdapter(logger shared_ports.Logger) ports.CommandExecutorPort {
	return &OSExecAdapter{
		logger:   logger,
		sessions: make(map[string]*shellSession),
	}
}

// Execute performs the raw process fork/exec.
// Since this is the ExecAdapter, it assumes SecurityGate/Warden have ALREADY approved this task.
func (a *OSExecAdapter) Execute(ctx context.Context, task domain.OSTask) (domain.OSTaskResult, error) {
	sessionID, err := a.Start(ctx, task)
	if err != nil {
		return domain.OSTaskResult{ExitCode: -1}, err
	}

	a.mu.RLock()
	session := a.sessions[sessionID]
	a.mu.RUnlock()

	ch, _ := a.Subscribe(ctx, sessionID)
	var stdout, stderr strings.Builder
	
	done := make(chan struct{})
	go func() {
		for output := range ch {
			if output.IsStderr {
				stderr.Write(output.Data)
			} else {
				stdout.Write(output.Data)
			}
		}
		close(done)
	}()

	<-done

	a.mu.RLock()
	defer a.mu.RUnlock()

	return domain.OSTaskResult{
		Status:     domain.StatusCompleted,
		Stdout:     strings.TrimSpace(stdout.String()),
		Stderr:     strings.TrimSpace(stderr.String()),
		ExitCode:   session.ExitCode,
		DurationMs: time.Since(session.StartedAt).Milliseconds(),
		SessionID:  sessionID,
	}, nil
}

func (a *OSExecAdapter) Start(ctx context.Context, task domain.OSTask) (string, error) {
	sessionID := uuid.New().String()
	
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, task.OriginalCmd, task.Args...)
	
	if task.Cwd != "" {
		cmd.Dir = task.Cwd
	}

	if len(task.Env) > 0 {
		env := os.Environ()
		for k, v := range task.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	session := &shellSession{
		ShellSession: domain.ShellSession{
			ID:        sessionID,
			Command:   task.OriginalCmd,
			Args:      task.Args,
			Cwd:       task.Cwd,
			StartedAt: time.Now(),
			IsActive:  true,
		},
		cmd:    cmd,
		cancel: cancel,
	}

	a.mu.Lock()
	a.sessions[sessionID] = session
	a.mu.Unlock()

	var rStdout io.ReadCloser
	var rStderr io.ReadCloser

	if task.UsePTY && runtime.GOOS != "windows" {
		f, err := pty.Start(cmd)
		if err != nil {
			return "", err
		}
		session.pty = f
		rStdout = f
		
		if task.Cols > 0 && task.Rows > 0 {
			_ = pty.Setsize(f, &pty.Winsize{
				Cols: uint16(task.Cols),
				Rows: uint16(task.Rows),
			})
		}
	} else {
		var err error
		rStdout, err = cmd.StdoutPipe()
		if err != nil {
			return "", err
		}
		rStderr, err = cmd.StderrPipe()
		if err != nil {
			return "", err
		}
		
		if err := cmd.Start(); err != nil {
			return "", err
		}
	}

	go a.handleOutput(session, rStdout, false)
	if rStderr != nil {
		go a.handleOutput(session, rStderr, true)
	}

	go func() {
		err := cmd.Wait()
		a.mu.Lock()
		session.IsActive = false
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				session.ExitCode = exitErr.ExitCode()
			} else {
				session.ExitCode = -1
			}
		}
		a.mu.Unlock()
		
		session.mu.Lock()
		for _, ch := range session.subs {
			close(ch)
		}
		session.subs = nil
		session.mu.Unlock()
		
		// B3 Fix: Prevent unbounded map growth by cleaning up session after TTL
		time.AfterFunc(5*time.Minute, func() {
			a.mu.Lock()
			delete(a.sessions, sessionID)
			a.mu.Unlock()
		})
	}()

	return sessionID, nil
}

func (a *OSExecAdapter) handleOutput(session *shellSession, r io.Reader, isStderr bool) {
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			output := domain.ShellOutput{
				SessionID: session.ID,
				Data:      append([]byte(nil), buf[:n]...),
				IsStderr:  isStderr,
				Timestamp: time.Now(),
			}
			
			session.mu.Lock()
			for _, ch := range session.subs {
				select {
				case ch <- output:
				default:
				}
			}
			session.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
}

func (a *OSExecAdapter) Kill(ctx context.Context, sessionID string) error {
	a.mu.RLock()
	session, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	
	if !ok {
		return fmt.Errorf("session not found")
	}
	
	session.cancel()
	if session.pty != nil {
		_ = session.pty.Close()
	}
	
	return nil
}

func (a *OSExecAdapter) Resize(ctx context.Context, sessionID string, cols, rows int) error {
	a.mu.RLock()
	session, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	
	if !ok {
		return fmt.Errorf("session not found")
	}
	
	if session.pty != nil {
		return pty.Setsize(session.pty, &pty.Winsize{
			Cols: uint16(cols),
			Rows: uint16(rows),
		})
	}
	
	return nil
}

func (a *OSExecAdapter) Subscribe(ctx context.Context, sessionID string) (<-chan domain.ShellOutput, error) {
	a.mu.RLock()
	session, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	
	ch := make(chan domain.ShellOutput, 100)
	
	session.mu.Lock()
	if !session.IsActive {
		session.mu.Unlock()
		close(ch)
		return ch, nil
	}
	session.subs = append(session.subs, ch)
	session.mu.Unlock()
	
	return ch, nil
}

func (a *OSExecAdapter) GetSession(ctx context.Context, sessionID string) (domain.ShellSession, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	session, ok := a.sessions[sessionID]
	if !ok {
		return domain.ShellSession{}, fmt.Errorf("session not found")
	}
	
	return session.ShellSession, nil
}

func (a *OSExecAdapter) ListSessions(ctx context.Context) ([]domain.ShellSession, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	sessions := make([]domain.ShellSession, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s.ShellSession)
	}
	
	return sessions, nil
}
