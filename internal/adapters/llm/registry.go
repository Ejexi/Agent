package llm

import (
	"duckops/internal/config"
	"duckops/internal/ports"
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

// RegisterFromConfig iterates through the provided configuration and registers all LLMs.
// It uses the OpenAICompatibleAdapter for any provider that has a BaseURL,
// and specialized adapters for known providers.
func (r *RegistryAdapter) RegisterFromConfig(cfgs map[string]config.LLMConfig) {
	for name, cfg := range cfgs {
		if cfg.APIKey == "" && name != "lmstudio" {
			continue // Skip if no API key provided (except for local LMStudio)
		}

		switch name {
		case "openai":
			r.Register(NewOpenAIAdapter(cfg.APIKey, cfg.Model))
		case "openrouter":
			r.Register(NewOpenRouterAdapter(cfg.APIKey, cfg.Model))
		case "lmstudio":
			r.Register(NewLMStudioAdapter(cfg.APIKey, cfg.Model, cfg.BaseURL))
		case "gemini":
			// Gemini is handled separately in InitApp due to context requirement
			continue
		default:
			// Treat everything else with a BaseURL as a custom compatible provider
			if cfg.BaseURL != "" {
				r.Register(NewOpenAICompatibleAdapter(name, cfg.APIKey, cfg.Model, cfg.BaseURL))
			}
		}
	}
}
