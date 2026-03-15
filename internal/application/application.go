package application

import (
	"context"
	"sync"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/types"
	"github.com/google/uuid"
)

// SessionManagerService implements ports.AppSessionManager.
type SessionManagerService struct {
	logger   shared_ports.Logger
	eventBus ports.EventBusPort
}

// NewSessionManagerService creates a new session manager service.
func NewSessionManagerService(l shared_ports.Logger, eb ports.EventBusPort) *SessionManagerService {
	return &SessionManagerService{
		logger:   l,
		eventBus: eb,
	}
}

// CreateSession initializes a new workspace session.
func (s *SessionManagerService) CreateSession(ctx context.Context, cwd string, mode string, model string) (string, error) {
	sessionID := uuid.New().String()
	s.logger.Info(ctx, "Workspace session created", shared_ports.Field{Key: "session_id", Value: sessionID}, shared_ports.Field{Key: "mode", Value: mode})

	err := s.eventBus.Publish(ctx, "session.created", map[string]string{
		"session_id": sessionID,
		"mode":       mode,
		"model":      model,
	})
	if err != nil {
		s.logger.ErrorErr(ctx, err, "Failed to publish session creation event")
	}

	return sessionID, nil
}

// Close shuts down the session manager.
func (s *SessionManagerService) Close() error {
	return nil
}

var _ ports.AppSessionManager = (*SessionManagerService)(nil)

// ToolRegistryService implements ports.ToolRegistry.
type ToolRegistryService struct {
	logger shared_ports.Logger
	mu     sync.RWMutex
	tools  map[string]domain.Tool
}

// NewToolRegistryService creates a new tool registry service.
func NewToolRegistryService(l shared_ports.Logger) *ToolRegistryService {
	return &ToolRegistryService{
		logger: l,
		tools:  make(map[string]domain.Tool),
	}
}

// RegisterTool adds a tool to the registry.
func (s *ToolRegistryService) RegisterTool(ctx context.Context, tool domain.Tool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tools[tool.Name()]; exists {
		return types.Newf(types.ErrCodeAlreadyExists, "tool already registered: %s", tool.Name())
	}

	s.tools[tool.Name()] = tool
	s.logger.Debug(ctx, "Tool registered", shared_ports.Field{Key: "tool", Value: tool.Name()})
	return nil
}

// GetTool retrieves a tool by name.
func (s *ToolRegistryService) GetTool(ctx context.Context, name string) (domain.Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tool, exists := s.tools[name]
	if !exists {
		return nil, types.Newf(types.ErrCodeToolNotFound, "tool not found: %s", name)
	}
	return tool, nil
}

// ListTools returns all tool schemas.
func (s *ToolRegistryService) ListTools(ctx context.Context) ([]domain.ToolSchema, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schemas := make([]domain.ToolSchema, 0, len(s.tools))
	for _, tool := range s.tools {
		schemas = append(schemas, tool.Schema())
	}
	return schemas, nil
}

var _ ports.ToolRegistry = (*ToolRegistryService)(nil)
