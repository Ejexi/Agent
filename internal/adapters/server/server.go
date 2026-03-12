package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/subagent"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
)

// AgentServer is the HTTP API for managing subagent sessions.
// Delegates to the Stand Duck  subagent.Tracker — no kernel dependency.
type AgentServer struct {
	sessions ports.SessionManager
	server   *http.Server
	addr     string
	logger   shared_ports.Logger
}

// NewAgentServer creates a new agent server.
func NewAgentServer(sessions ports.SessionManager, addr string, logger shared_ports.Logger) *AgentServer {
	return &AgentServer{
		sessions: sessions,
		addr:     addr,
		logger:   logger,
	}
}

// Start begins listening for HTTP requests.
func (s *AgentServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	mux.HandleFunc("GET /v1/sessions/", s.handleSessionRoutes)
	mux.HandleFunc("POST /v1/sessions/", s.handlePostSessionRoutes)
	mux.HandleFunc("DELETE /v1/sessions/", s.handleDeleteSession)

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
		Description  string   `json:"description"`
		Instructions string   `json:"instructions"`
		Context      string   `json:"context,omitempty"`
		Tools        []string `json:"tools"`
		Model        string   `json:"model,omitempty"`
		MaxSteps     int      `json:"max_steps,omitempty"`
		Sandbox      bool     `json:"enable_sandbox,omitempty"`
		MaxRetries   int      `json:"max_retries,omitempty"`
		Provider     string   `json:"provider,omitempty"`
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
	}

	if req.MaxRetries > 0 {
		config.Retry = subagent.RetryPolicy{MaxRetries: req.MaxRetries, DelayMs: 1000}
	}
	config.ApplyDefaults()

	sessionID, err := s.sessions.SpawnSubagent("", config)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"session_id": sessionID,
		"status":     string(subagent.StatusPending),
	})
}

func (s *AgentServer) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.sessions.ListSessions()
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

	if len(parts) == 2 && parts[1] == "events" {
		s.handleStreamEvents(w, r, sessionID)
		return
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

	if len(parts) == 2 && parts[1] == "resume" {
		s.handleResumeSession(w, r, sessionID)
		return
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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
