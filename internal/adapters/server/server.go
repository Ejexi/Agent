package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	checkpoint_store "github.com/SecDuckOps/agent/internal/adapters/checkpoint"
	"github.com/SecDuckOps/agent/internal/domain/subagent"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
)

// AgentServer is the HTTP API for managing subagent sessions.
// Delegates to the Stand Duck  subagent.Tracker — no kernel dependency.
type AgentServer struct {
	sessions        ports.SessionManager
	checkpoints     *checkpoint_store.Store // optional — nil disables checkpoint API
	server          *http.Server
	addr            string
	logger          shared_ports.Logger
}

// NewAgentServer creates a new agent server.
func NewAgentServer(sessions ports.SessionManager, addr string, logger shared_ports.Logger) *AgentServer {
	return &AgentServer{
		sessions: sessions,
		addr:     addr,
		logger:   logger,
	}
}

// WithCheckpointStore attaches the checkpoint store so /v1/checkpoints is available.
func (s *AgentServer) WithCheckpointStore(store *checkpoint_store.Store) *AgentServer {
	s.checkpoints = store
	return s
}

// Start begins listening for HTTP requests.
func (s *AgentServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	mux.HandleFunc("GET /v1/sessions/", s.handleSessionRoutes)
	mux.HandleFunc("POST /v1/sessions/", s.handlePostSessionRoutes)
	mux.HandleFunc("DELETE /v1/sessions/", s.handleDeleteSession)

	// Checkpoint endpoints — session history persistence
	mux.HandleFunc("GET /v1/checkpoints", s.handleListCheckpoints)
	mux.HandleFunc("GET /v1/checkpoints/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/checkpoints/")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing session id"})
			return
		}
		s.handleGetCheckpoint(w, r, id)
	})
	mux.HandleFunc("DELETE /v1/checkpoints/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/checkpoints/")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing session id"})
			return
		}
		s.handleDeleteCheckpoint(w, r, id)
	})

	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // SSE requires no write timeout
	}

	if s.logger != nil {
		s.logger.Info(context.Background(), "AgentServer Listening", shared_ports.Field{Key: "addr", Value: s.addr})
	}
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *AgentServer) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// --- HTTP Handlers ---

func (s *AgentServer) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description     string   `json:"description"`
		Instructions    string   `json:"instructions"`
		Context         string   `json:"context,omitempty"`
		Tools           []string `json:"tools"`
		Model           string   `json:"model,omitempty"`
		MaxSteps        int      `json:"max_steps,omitempty"`
		Sandbox         bool     `json:"enable_sandbox,omitempty"`
		MaxRetries      int      `json:"max_retries,omitempty"`
		Provider        string   `json:"provider,omitempty"`
		ParentSessionID string   `json:"parent_session_id,omitempty"`
		AutoApprove     bool     `json:"auto_approve,omitempty"`
		// CheckpointID resumes a previous session from its saved message history.
		// Mirrors duckops CLI --checkpoint flag.
		CheckpointID string `json:"checkpoint_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Instructions == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "instructions is required"})
		return
	}

	config := subagent.SessionConfig{
		Description:  req.Description,
		Instructions: req.Instructions,
		Context:      req.Context,
		AllowedTools: req.Tools,
		Model:        req.Model,
		MaxSteps:     req.MaxSteps,
		Sandbox:      req.Sandbox,
		Provider:     req.Provider,
		CheckpointID: req.CheckpointID,
	}

	// Gap 3: auto_approve overrides PauseOnApproval (sandbox/unattended mode)
	if req.AutoApprove {
		config.PauseOnApproval = false
		config.Sandbox = true
	}

	if req.MaxRetries > 0 {
		config.Retry = subagent.RetryPolicy{MaxRetries: req.MaxRetries, DelayMs: 1000}
	}
	config.ApplyDefaults()

	// Gap 1: pass parent_session_id to tracker (enables parent-child linking)
	parentID := req.ParentSessionID
	sessionID, err := s.sessions.SpawnSubagent(parentID, config)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"session_id":        sessionID,
		"parent_session_id": parentID,
		"status":            string(subagent.StatusPending),
	})
}

func (s *AgentServer) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.sessions.ListSessions()

	// Gap 1: filter by parent_session_id if provided
	// GET /v1/sessions?parent_session_id=<id>
	if parentID := r.URL.Query().Get("parent_session_id"); parentID != "" {
		filtered := sessions[:0]
		for _, sa := range sessions {
			if sa.ParentID == parentID {
				filtered = append(filtered, sa)
			}
		}
		sessions = filtered
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

func (s *AgentServer) handleSessionRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	parts := strings.SplitN(path, "/", 2)

	sessionID := parts[0]
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id is required"})
		return
	}

	if len(parts) == 2 {
		switch parts[1] {
		case "events":
			s.handleStreamEvents(w, r, sessionID)
			return
		case "children":
			// Gap 1: list all subagent sessions spawned by this session
			s.handleListChildren(w, r, sessionID)
			return
		case "tools/pending":
			// Gap 3: get pending tool call approvals
			s.handleGetPendingTools(w, r, sessionID)
			return
		}
	}

	s.handleGetSession(w, r, sessionID)
}

func (s *AgentServer) handleGetSession(w http.ResponseWriter, _ *http.Request, sessionID string) {
	view, err := s.sessions.GetSession(sessionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	// Strip instructions from response
	sa := view.Subagent
	sa.Config.Instructions = ""
	writeJSON(w, http.StatusOK, sa)
}

func (s *AgentServer) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id is required"})
		return
	}

	if err := s.sessions.CancelSession(sessionID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *AgentServer) handlePostSessionRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	parts := strings.SplitN(path, "/", 2)

	sessionID := parts[0]
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id is required"})
		return
	}

	if len(parts) == 2 {
		switch parts[1] {
		case "resume":
			s.handleResumeSession(w, r, sessionID)
			return
		case "cancel":
			// Gap 3: explicit cancel via POST (mirrors duckops POST /v1/sessions/{id}/cancel)
			if err := s.sessions.CancelSession(sessionID); err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
			return
		case "tools/decisions":
			// Gap 3: batch approve/reject tool calls (mirrors duckops POST .../tools/decisions)
			s.handleToolDecisions(w, r, sessionID)
			return
		case "command":
			// Feature 3: runtime AgentCommand — steering, follow_up, switch_model, cancel
			s.handleSendCommand(w, r, sessionID)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (s *AgentServer) handleResumeSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req struct {
		Approve    []string `json:"approve,omitempty"`
		Reject     []string `json:"reject,omitempty"`
		ApproveAll bool     `json:"approve_all,omitempty"`
		RejectAll  bool     `json:"reject_all,omitempty"`
		Input      string   `json:"input,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	decision := subagent.ResumeDecision{
		Approve:    req.Approve,
		Reject:     req.Reject,
		ApproveAll: req.ApproveAll,
		RejectAll:  req.RejectAll,
		Input:      req.Input,
	}

	if err := s.sessions.ResumeSession(sessionID, decision); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

func (s *AgentServer) handleStreamEvents(w http.ResponseWriter, r *http.Request, sessionID string) {
	subID, events, err := s.sessions.StreamEvents(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer s.sessions.UnsubscribeEvents(sessionID, subID)

	// Replay buffered events first (catch-up)
	replayed, _ := s.sessions.ReplayEvents(sessionID, 0)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send replayed events
	for _, indexed := range replayed {
		data, _ := json.Marshal(indexed.Event)
		fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", indexed.SeqID, indexed.Event.Type, data)
		flusher.Flush()
	}

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case indexed, ok := <-events:
			if !ok {
				data, _ := json.Marshal(map[string]string{
					"type":    "session_ended",
					"message": "Session has completed",
				})
				fmt.Fprintf(w, "event: session_ended\ndata: %s\n\n", data)
				flusher.Flush()
				return
			}
			data, _ := json.Marshal(indexed.Event)
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", indexed.SeqID, indexed.Event.Type, data)
			flusher.Flush()
		}
	}
}

// handleListChildren returns all subagent sessions whose ParentID matches sessionID.
// Gap 1: mirrors duckops's list_sessions?parent_session_id= filter.
func (s *AgentServer) handleListChildren(w http.ResponseWriter, _ *http.Request, sessionID string) {
	all := s.sessions.ListSessions()
	children := all[:0]
	for _, sa := range all {
		if sa.ParentID == sessionID {
			children = append(children, sa)
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"parent_session_id": sessionID,
		"children":          children,
		"count":             len(children),
	})
}

// handleGetPendingTools returns tool calls awaiting approval for the session.
// Gap 3: mirrors duckops GET /v1/sessions/{id}/tools/pending
func (s *AgentServer) handleGetPendingTools(w http.ResponseWriter, _ *http.Request, sessionID string) {
	view, err := s.sessions.GetSession(sessionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	pending := []interface{}{}
	if view.Subagent.PauseInfo != nil {
		for _, tc := range view.Subagent.PauseInfo.PendingToolCalls {
			pending = append(pending, map[string]interface{}{
				"id":   tc.ID,
				"name": tc.Name,
				"args": tc.Args,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id":    sessionID,
		"pending_tools": pending,
		"count":         len(pending),
	})
}

// handleToolDecisions processes batch approve/reject decisions for pending tool calls.
// Gap 3: mirrors duckops POST /v1/sessions/{id}/tools/decisions
// This is a semantic alias to resume — the same ResumeDecision struct covers approvals.
func (s *AgentServer) handleToolDecisions(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req struct {
		Approve    []string `json:"approve,omitempty"`
		Reject     []string `json:"reject,omitempty"`
		ApproveAll bool     `json:"approve_all,omitempty"`
		RejectAll  bool     `json:"reject_all,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	decision := subagent.ResumeDecision{
		Approve:    req.Approve,
		Reject:     req.Reject,
		ApproveAll: req.ApproveAll,
		RejectAll:  req.RejectAll,
	}

	if err := s.sessions.ResumeSession(sessionID, decision); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "decisions_applied"})
}

// handleSendCommand delivers a runtime AgentCommand to a running session.
// POST /v1/sessions/{id}/command
// Feature 3: Steering, FollowUp, SwitchModel, Cancel mid-run.
func (s *AgentServer) handleSendCommand(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req struct {
		Type    string `json:"type"`    // "steering" | "follow_up" | "switch_model" | "cancel"
		Payload string `json:"payload"` // text / model name
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	cmd := subagent.AgentCommand{
		Type:    subagent.AgentCommandType(req.Type),
		Payload: req.Payload,
	}

	if err := s.sessions.SendCommand(sessionID, cmd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "command_sent",
		"type":       req.Type,
		"session_id": sessionID,
	})
}

// handleListCheckpoints returns all saved session checkpoints (most recent first).
// GET /v1/checkpoints
func (s *AgentServer) handleListCheckpoints(w http.ResponseWriter, _ *http.Request) {
	if s.checkpoints == nil {
		writeJSON(w, http.StatusServiceUnavailable,
			map[string]string{"error": "checkpoint store not configured"})
		return
	}
	sessions, err := s.checkpoints.ListSessions()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type entry struct {
		SessionID string `json:"session_id"`
		Turns     int    `json:"turns"`
		UpdatedAt string `json:"updated_at"`
		CreatedAt string `json:"created_at"`
	}
	result := make([]entry, len(sessions))
	for i, s := range sessions {
		result[i] = entry{
			SessionID: s.SessionID,
			Turns:     len(s.Messages),
			UpdatedAt: s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// handleGetCheckpoint returns one session checkpoint by ID.
// GET /v1/checkpoints/{id}
func (s *AgentServer) handleGetCheckpoint(w http.ResponseWriter, r *http.Request, sessionID string) {
	if s.checkpoints == nil {
		writeJSON(w, http.StatusServiceUnavailable,
			map[string]string{"error": "checkpoint store not configured"})
		return
	}
	env, err := s.checkpoints.Load(r.Context(), sessionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, env)
}

// handleDeleteCheckpoint deletes a saved checkpoint.
// DELETE /v1/checkpoints/{id}
func (s *AgentServer) handleDeleteCheckpoint(w http.ResponseWriter, r *http.Request, sessionID string) {
	if s.checkpoints == nil {
		writeJSON(w, http.StatusServiceUnavailable,
			map[string]string{"error": "checkpoint store not configured"})
		return
	}
	if err := s.checkpoints.Delete(r.Context(), sessionID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "session_id": sessionID})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
