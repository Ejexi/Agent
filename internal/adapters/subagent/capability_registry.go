package subagent

import (
	"sync"

	"github.com/SecDuckOps/agent/internal/ports"
)

// CapabilityRegistry holds dynamically injected subagent profiles (capabilities).
type CapabilityRegistry struct {
	mu           sync.RWMutex
	capabilities map[string]ports.Capability
}

// NewCapabilityRegistry creates a new empty registry.
func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{
		capabilities: make(map[string]ports.Capability),
	}
}

// Sync overwrites the registry with a new list of capabilities fetched from the cloud.
func (r *CapabilityRegistry) Sync(caps []ports.Capability) {
	newCaps := make(map[string]ports.Capability, len(caps))
	for _, c := range caps {
		newCaps[c.Name] = c
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.capabilities = newCaps
}

// Get returns a specific capability by name.
func (r *CapabilityRegistry) Get(name string) (ports.Capability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.capabilities[name]
	return c, ok
}

// List returns all registered capabilities.
func (r *CapabilityRegistry) List() []ports.Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]ports.Capability, 0, len(r.capabilities))
	for _, c := range r.capabilities {
		list = append(list, c)
	}
	return list
}
