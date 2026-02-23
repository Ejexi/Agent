package kernel

import (
	"bufio"
	"github.com/SecDuckOps/Agent/internal/config"
	"github.com/SecDuckOps/Agent/internal/ports"
	"github.com/SecDuckOps/Shared/types"
	"fmt"
	"os"
	"strings"
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

// ConfigureCustomProvider handles the interactive addition of a new provider.
func (s *SetupService) ConfigureCustomProvider(appConfig *config.Config) error {
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
	appConfig.LLMs[name] = llmCfg

	// Save to config.yaml (via config pkg)
	if err := config.SaveConfig("config.yaml", appConfig); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to save to config.yaml")
	}

	fmt.Printf("\nSuccessfully added provider '%s' to config.yaml!\n", name)
	return nil
}
