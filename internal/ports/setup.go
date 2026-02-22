package ports

import "duckops/internal/config"

// SetupRepository handles the persistence of the agent configuration.
type SetupRepository interface {
	Load() (*config.Config, error)
	Save(cfg *config.Config) error
}

// SetupPrompter handles user interaction for setup procedures.
type SetupPrompter interface {
	SelectProvider(providers []string) (string, error)
	PromptCustomProvider() (name string, cfg config.LLMConfig, err error)
}
