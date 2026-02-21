package llm

import (
	"agent/internal/ports"
	"sync"
)

// RegistryAdapter implements the ports.LLMRegistry interface.
type RegistryAdapter struct {
	llms            map[string]ports.LLM
	defaultProvider string
	mu              sync.RWMutex
}

// NewRegistryAdapter creates a new thread-safe LLM registry with a fallback provider.
func NewRegistryAdapter(defaultProvider string) *RegistryAdapter {
	if defaultProvider == "" {
		defaultProvider = "default"
	}
	return &RegistryAdapter{
		llms:            make(map[string]ports.LLM),
		defaultProvider: defaultProvider,
	}
}

// Register adds a new LLM provider to the registry.
func (r *RegistryAdapter) Register(llm ports.LLM) {
	if llm == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.llms[llm.Name()] = llm
}

// Get returns the registered LLM provider by name, with O(1) fallback capability.
func (r *RegistryAdapter) Get(name string) ports.LLM {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Direct match
	if provider, exists := r.llms[name]; exists {
		return provider
	}

	// 2. Fallback to default avoiding double locking
	return r.llms[r.defaultProvider]
}

// List returns all registered LLM provider names efficiently.
func (r *RegistryAdapter) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Avoid growing the slice dynamically inside the loop
	names := make([]string, 0, len(r.llms))
	for k := range r.llms {
		names = append(names, k)
	}
	return names
}
