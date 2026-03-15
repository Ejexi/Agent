package ports

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
)

// AppSessionManager defines the interface for managing top-level application sessions.
type AppSessionManager interface {
	// CreateSession initializes a new workspace session for the user.
	CreateSession(ctx context.Context, cwd string, mode string, model string) (string, error)

	// Close shuts down all active sessions.
	Close() error
}

// ToolRegistry defines the interface for registering and looking up tools.
type ToolRegistry interface {
	// RegisterTool adds a new tool to the registry.
	RegisterTool(ctx context.Context, tool domain.Tool) error

	// GetTool retrieves a tool by its name.
	GetTool(ctx context.Context, name string) (domain.Tool, error)

	// ListTools returns the schemas of all registered tools.
	ListTools(ctx context.Context) ([]domain.ToolSchema, error)
}
