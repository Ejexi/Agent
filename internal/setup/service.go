package setup

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/SecDuckOps/agent/internal/config"
	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/shared/types"
)

// Repository handles the persistence of the agent configuration.
type Repository interface {
	Load() (*config.Config, error)
	Save(cfg *config.Config) error
}

// Prompter handles user interaction for setup procedures.
type Prompter interface {
	SelectProvider(providers []string) (string, error)
	PromptCustomProvider() (name string, cfg domain.ProviderConfig, err error)
}

// Service handles the first-run configuration logic for the Agent.
// This belongs in the application layer — not in the kernel.
type Service struct {
	repo     Repository
	prompter Prompter
}

func NewService(repo Repository, prompter Prompter) *Service {
	return &Service{
		repo:     repo,
		prompter: prompter,
	}
}

// GetProvider loads the setup config; if configured returns it, otherwise prompts user and saves.
func (s *Service) GetProvider(providers []string) (string, error) {
	cfg, err := s.repo.Load()

	// If loaded successfully and already configured → return silently
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

// ConfigureCustomProvider handles the interactive addition of a new provider.
func (s *Service) ConfigureCustomProvider(appConfig *config.Config) error {
	fmt.Print("\nWould you like to add a new custom LLM provider? (y/n): ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if answer != "y" && answer != "yes" {
			return nil
		}
	}

	name, llmCfg, err := s.prompter.PromptCustomProvider()
	if err != nil {
		return err
	}

	if appConfig.LLMs == nil {
		appConfig.LLMs = make(map[string]config.LLMConfig)
	}
	appConfig.LLMs[name] = config.LLMConfig{
		APIKey:  llmCfg.APIKey,
		Model:   llmCfg.Model,
		BaseURL: llmCfg.BaseURL,
	}

	if err := s.repo.Save(appConfig); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to save configuration")
	}

	fmt.Printf("\nSuccessfully added provider '%s'!\n", name)
	return nil
}
