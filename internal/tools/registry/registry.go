package registry

//The system can see and use the tools form overthere
import (
	"fmt"
	"sync"

	"github.com/Ejexi/Agent/internal/tools/base"
)

// manages all available tools
type Registry struct {
	tools map[string]base.Tool // tool_name -> tool instance
	mu    sync.RWMutex
}

// Registy Constructor start with no tools
func New() *Registry {
	return &Registry{
		tools: make(map[string]base.Tool),
	}
}

// adds a tool to the registry and prevent duplication
func (r *Registry) Register(tool base.Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()

	// Check if tool already exists
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}
	// Add the tool
	r.tools[name] = tool
	return nil
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (base.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return tool, nil
}

// List returns all registered tools
func (r *Registry) List() []base.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]base.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Has checks if a tool exists ex: validation
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.tools[name]
	return exists
}

// Count returns the number of registered tools
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools)
}
