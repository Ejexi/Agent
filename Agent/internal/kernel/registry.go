package kernel

import (
	types "agent/internal/Types"
	"agent/internal/domain"
	"sync"
)

// Registry manages the available tools.
type Registry struct {
	tools map[string]domain.Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]domain.Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool domain.Tool) error {
	if tool == nil {
		return types.New(types.ErrCodeInvalidInput, "cannot register nil tool")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[tool.Name()]; exists {
		return types.Newf(types.ErrCodeInvalidInput, "tool %s is already registered", tool.Name())
	}
	r.tools[tool.Name()] = tool
	return nil
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (domain.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}
