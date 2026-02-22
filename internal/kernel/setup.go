package kernel

import (
	"agent/internal/config"
	"agent/internal/ports"
	"agent/internal/types"
)

// SetupService handles the first-run configuration logic for the Agent.
type SetupService struct {
	repo     ports.SetupRepository
	prompter ports.SetupPrompter
}

func NewSetupService(repo ports.SetupRepository, prompter ports.SetupPrompter) *SetupService {
	return &SetupService{
		repo:     repo,
		prompter: prompter,
	}
}

// GetProvider loads the setup config, if it exists returns it, otherwise prompts user and saves.
func (s *SetupService) GetProvider(providers []string) (string, error) {
	cfg, err := s.repo.Load()

	// If loaded successfully and already configured -> return silently
	if err == nil && cfg != nil && cfg.Provider != "" {
		return cfg.Provider, nil
	}

	// Otherwise, first-run: ask the user
	selectedProvider, err := s.prompter.SelectProvider(providers)
	if err != nil {
		return "", types.Wrap(err, types.ErrCodeInternal, "failed to select provider")
	}

	if cfg == nil {
		cfg = &config.Config{}
	}
	cfg.Provider = selectedProvider

	if err := s.repo.Save(cfg); err != nil {
		return "", types.Wrap(err, types.ErrCodeInternal, "failed to save first-run configuration")
	}

	return selectedProvider, nil
}
