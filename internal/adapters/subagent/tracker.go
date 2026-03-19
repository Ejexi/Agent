package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	sa "github.com/SecDuckOps/agent/internal/domain/subagent"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/types"
	"github.com/google/uuid"
)

// SubagentSession holds the runtime state for a single subagent session.
type SubagentSession struct {
	Subagent   sa.Subagent
	Log        *EventLog              // Durable event log (replaces EventChan)
	ResumeChan chan sa.ResumeDecision // Channel for receiving resume decisions
	Ctx        context.Context
	Cancel     context.CancelFunc
	mu         sync.RWMutex
}

// Emit sends an event to the session's EventLog.
func (s *SubagentSession) Emit(evt sa.SubagentEvent) {
	s.mu.RLock()
	evt.SessionID = s.Subagent.SessionID
	evt.RunID = s.Subagent.RunID
	s.mu.RUnlock()

	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}

	s.Log.Append(evt)

	// Propagate to ExecutionContext if available (Phase 6: Visual Learning)
	if e, ok := s.Ctx.(interface{ Emit(any) }); ok {
		e.Emit(evt)
	}
}

// SetStatus transitions the session status and emits a status_change event.
func (s *SubagentSession) SetStatus(status sa.SubagentStatus) {
	s.mu.Lock()
	s.Subagent.Status = status

	now := time.Now()
	switch status {
	case sa.StatusRunning:
		s.Subagent.StartedAt = &now
		s.Subagent.RunState = sa.RunStateRunning
	case sa.StatusCompleted, sa.StatusCancelled:
		s.Subagent.CompletedAt = &now
		s.Subagent.RunState = sa.RunStateIdle
	case sa.StatusFailed:
		s.Subagent.CompletedAt = &now
		s.Subagent.RunState = sa.RunStateFailed
	}
	s.mu.Unlock()

	s.Emit(sa.SubagentEvent{
		Type:    sa.EventStatus,
		Message: fmt.Sprintf("status changed to %s", status),
	})
}

// SetPauseInfo sets the pause info and transitions to paused status.
func (s *SubagentSession) SetPauseInfo(info *sa.PauseInfo) {
	s.mu.Lock()
	s.Subagent.PauseInfo = info
	s.Subagent.Status = sa.StatusPaused
	s.mu.Unlock()

	s.Emit(sa.SubagentEvent{
		Type:    sa.EventPaused,
		Message: fmt.Sprintf("Paused: %s", info.Reason),
		Data:    info,
	})
}

// Tracker manages all active subagent sessions.
// Completely decoupled from the Kernel — the Kernel only executes tools.
type Tracker struct {
	sessions          map[string]*SubagentSession
	executor          ports.ToolExecutor
	schemaProvider    ports.ToolSchemaProvider
	secretScanner     ports.SecretScannerPort
	logger            shared_ports.Logger
	OnSessionComplete func(sessionID string) // Optional hook for session cleanup
	mu                sync.RWMutex
}

// NewTracker creates a new subagent tracker.
func NewTracker(executor ports.ToolExecutor, schemaProvider ports.ToolSchemaProvider, secretScanner ports.SecretScannerPort, logger shared_ports.Logger) *Tracker {
	return &Tracker{
		sessions:       make(map[string]*SubagentSession),
		executor:       executor,
		schemaProvider: schemaProvider,
		secretScanner:  secretScanner,
		logger:         logger,
	}
}

// SpawnSubagent creates a new session and starts the agent loop in a goroutine.
func (t *Tracker) SpawnSubagent(parentID string, config sa.SessionConfig) (string, error) {
	depth := 0
	if parentID != "" {
		parent, err := t.GetSession(parentID)
		if err == nil {
			depth = parent.Subagent.Depth + 1
		}
		if depth > 3 {
			return "", types.Newf(types.ErrCodePermissionDenied, "maximum subagent depth exceeded (limit: 3) to prevent recursive runaway costs")
		}
	}
	return t.spawnWithRetry(parentID, "", config, 0, depth)
}

// spawnWithRetry creates a session, optionally linked to an original session for retries.
func (t *Tracker) spawnWithRetry(parentID string, originalID string, config sa.SessionConfig, retryCount int, depth int) (string, error) {
	sessionID := uuid.New().String()
	subagentID := uuid.New().String()
	runID := uuid.New().String()
	now := time.Now()

	// Apply defaults (single source of truth — domain/subagent)
	config.ApplyDefaults()

	// Background context — sessions outlive the spawning request
	var sessionCtx context.Context
	var cancel context.CancelFunc
	if config.TimeoutSeconds > 0 {
		sessionCtx, cancel = context.WithTimeout(context.Background(), time.Duration(config.TimeoutSeconds)*time.Second)
	} else {
		sessionCtx, cancel = context.WithCancel(context.Background())
	}

	if originalID == "" {
		originalID = subagentID
	}

	session := &SubagentSession{
		Subagent: sa.Subagent{
			ID:         subagentID,
			ParentID:   parentID,
			OriginalID: originalID,
			SessionID:  sessionID,
			RunID:      runID,
			Config:     config,
			Status:     sa.StatusPending,
			RunState:   sa.RunStateStarting,
			RetryCount: retryCount,
			Depth:      depth,
			CreatedAt:  now,
		},
		Log:        NewEventLog(sessionID),
		ResumeChan: make(chan sa.ResumeDecision, 1),
		Ctx:        sessionCtx,
		Cancel:     cancel,
	}

	t.mu.Lock()
	t.sessions[sessionID] = session
	t.mu.Unlock()

	go t.runSessionLoop(session)

	return sessionID, nil
}

// GetSession retrieves a session by ID.
func (t *Tracker) GetSession(sessionID string) (ports.SessionView, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	session, exists := t.sessions[sessionID]
	if !exists {
		return ports.SessionView{}, types.Newf(types.ErrCodeNotFound, "session not found: %s", sessionID)
	}

	session.mu.RLock()
	view := ports.SessionView{Subagent: session.Subagent}
	session.mu.RUnlock()

	return view, nil
}

// ListSessions returns all tracked sessions.
func (t *Tracker) ListSessions() []sa.Subagent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]sa.Subagent, 0, len(t.sessions))
	for _, s := range t.sessions {
		s.mu.RLock()
		result = append(result, s.Subagent)
		s.mu.RUnlock()
	}
	return result
}

// CancelSession terminates a running subagent session.
func (t *Tracker) CancelSession(sessionID string) error {
	t.mu.RLock()
	session, exists := t.sessions[sessionID]
	t.mu.RUnlock()

	if !exists {
		return types.Newf(types.ErrCodeNotFound, "session not found: %s", sessionID)
	}

	session.Cancel()
	session.SetStatus(sa.StatusCancelled)

	// Trigger hook
	if t.OnSessionComplete != nil {
		t.OnSessionComplete(sessionID)
	}

	return nil
}

// ResumeSession sends a resume decision to a paused session.
func (t *Tracker) ResumeSession(sessionID string, decision sa.ResumeDecision) error {
	t.mu.RLock()
	session, exists := t.sessions[sessionID]
	t.mu.RUnlock()

	if !exists {
		return types.Newf(types.ErrCodeNotFound, "session not found: %s", sessionID)
	}

	session.mu.RLock()
	status := session.Subagent.Status
	session.mu.RUnlock()

	if status != sa.StatusPaused {
		return types.Newf(types.ErrCodeInvalidInput, "session %s is not paused (status: %s)", sessionID, status)
	}

	select {
	case session.ResumeChan <- decision:
		return nil
	default:
		return types.Newf(types.ErrCodeInternal, "resume channel is full for session %s", sessionID)
	}
}

// StreamEvents returns a subscription to the session's EventLog.
// Returns the subscription ID (for unsubscribe) and a read-only channel of port-level indexed events.
func (t *Tracker) StreamEvents(sessionID string) (uint64, <-chan ports.IndexedEvent, error) {
	t.mu.RLock()
	session, exists := t.sessions[sessionID]
	t.mu.RUnlock()

	if !exists {
		return 0, nil, types.Newf(types.ErrCodeNotFound, "session not found: %s", sessionID)
	}

	subID, internalCh := session.Log.Subscribe()

	// Adapt internal IndexedEvent → ports.IndexedEvent
	portCh := make(chan ports.IndexedEvent, cap(internalCh))
	go func() {
		defer close(portCh)
		for evt := range internalCh {
			portCh <- ports.IndexedEvent{SeqID: evt.SeqID, Event: evt.Event}
		}
	}()

	return subID, portCh, nil
}

// UnsubscribeEvents removes an SSE subscriber from a session's EventLog.
func (t *Tracker) UnsubscribeEvents(sessionID string, subID uint64) {
	t.mu.RLock()
	session, exists := t.sessions[sessionID]
	t.mu.RUnlock()

	if !exists {
		return
	}

	session.Log.Unsubscribe(subID)
}

// ReplayEvents returns buffered events since the given sequence ID.
func (t *Tracker) ReplayEvents(sessionID string, sinceSeqID uint64) ([]ports.IndexedEvent, error) {
	t.mu.RLock()
	session, exists := t.sessions[sessionID]
	t.mu.RUnlock()

	if !exists {
		return nil, types.Newf(types.ErrCodeNotFound, "session not found: %s", sessionID)
	}

	internal := session.Log.Replay(sinceSeqID)
	result := make([]ports.IndexedEvent, len(internal))
	for i, evt := range internal {
		result[i] = ports.IndexedEvent{SeqID: evt.SeqID, Event: evt.Event}
	}
	return result, nil
}

// runSessionLoop is the core agent loop for a subagent.
func (t *Tracker) runSessionLoop(session *SubagentSession) {
	defer session.Log.Close()

	session.SetStatus(sa.StatusRunning)

	actor := NewSessionActor(t.executor, t.schemaProvider, t.secretScanner, session)
	err := actor.Run()

	if err != nil {
		session.mu.Lock()
		session.Subagent.Error = err.Error()
		retryCount := session.Subagent.RetryCount
		maxRetries := session.Subagent.Config.Retry.MaxRetries
		originalID := session.Subagent.OriginalID
		parentID := session.Subagent.ParentID
		config := session.Subagent.Config
		delayMs := session.Subagent.Config.Retry.DelayMs
		session.mu.Unlock()

		session.Emit(sa.SubagentEvent{
			Type:    sa.EventError,
			Message: err.Error(),
		})

		// Retry logic
		if retryCount < maxRetries {
			session.SetStatus(sa.StatusRetrying)

			session.Emit(sa.SubagentEvent{
				Type:    sa.EventRetry,
				Message: fmt.Sprintf("Retrying (%d/%d) linked to original %s", retryCount+1, maxRetries, originalID),
				Data: map[string]interface{}{
					"retry_count": retryCount + 1,
					"max_retries": maxRetries,
					"original_id": originalID,
				},
			})

			if delayMs > 0 {
				time.Sleep(time.Duration(delayMs) * time.Millisecond)
			}
			
			session.mu.Lock()
			currentDepth := session.Subagent.Depth
			session.mu.Unlock()

			newSessionID, retryErr := t.spawnWithRetry(parentID, originalID, config, retryCount+1, currentDepth)
			if retryErr != nil {
				if t.logger != nil {
					t.logger.ErrorErr(session.Ctx, retryErr, "Failed to spawn retry session", shared_ports.Field{Key: "original_id", Value: originalID})
				}
				session.SetStatus(sa.StatusFailed)
				if t.OnSessionComplete != nil {
					t.OnSessionComplete(session.Subagent.SessionID)
				}
			} else {
				if t.logger != nil {
					t.logger.Info(session.Ctx, "Spawned retry session",
						shared_ports.Field{Key: "new_session_id", Value: newSessionID},
						shared_ports.Field{Key: "attempt", Value: retryCount + 1},
						shared_ports.Field{Key: "original_id", Value: originalID})
				}
			}
			return
		}

		session.SetStatus(sa.StatusFailed)
		if t.OnSessionComplete != nil {
			t.OnSessionComplete(session.Subagent.SessionID)
		}
		return
	}

	session.SetStatus(sa.StatusCompleted)
	if t.OnSessionComplete != nil {
		t.OnSessionComplete(session.Subagent.SessionID)
	}
}
