package cli

import (
	"bufio"
	"github.com/SecDuckOps/Agent/internal/config"
	"github.com/SecDuckOps/Shared/types"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SetupPrompter implements the console logic for setup operations.
type SetupPrompter struct{}

func NewSetupPrompter() *SetupPrompter {
	return &SetupPrompter{}
}

func (p *SetupPrompter) SelectProvider(providers []string) (string, error) {
	if len(providers) == 0 {
		return "", types.New(types.ErrCodeInvalidInput, "no providers available")
	}

	defaultProvider := providers[0]
	// Require default gemini if it exists.
	for _, prov := range providers {
		if prov == "gemini" {
			defaultProvider = "gemini"
			break
		}
	}

	fmt.Printf("\n--- Available AI Providers ---\n")
	for i, p := range providers {
		fmt.Printf("[%d] %s\n", i+1, p)
	}

	fmt.Printf("Select AI Provider (default: %s): ", defaultProvider)

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input != "" {
			if idx, err := strconv.Atoi(input); err == nil && idx >= 1 && idx <= len(providers) {
				return providers[idx-1], nil
			}
			return input, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", types.Wrap(err, types.ErrCodeInternal, "failed to read user input from CLI")
	}

	return defaultProvider, nil
}

func (p *SetupPrompter) PromptCustomProvider() (name string, cfg config.LLMConfig, err error) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Printf("\n--- Add New Custom LLM Provider ---\n")

	for name == "" {
		fmt.Print("Provider Name (e.g., groq, ollama): ")
		if !scanner.Scan() {
			return "", cfg, types.New(types.ErrCodeInternal, "failed to read name")
		}
		name = strings.TrimSpace(scanner.Text())
		if name == "" {
			fmt.Println("Error: Provider name cannot be empty.")
		}
	}

	for cfg.APIKey == "" {
		fmt.Print("API Key (or env var like ${GROQ_API_KEY}): ")
		if !scanner.Scan() {
			return "", cfg, types.New(types.ErrCodeInternal, "failed to read api key")
		}
		cfg.APIKey = strings.TrimSpace(scanner.Text())
		if cfg.APIKey == "" {
			fmt.Println("Error: API Key is required (type 'none' if not needed).")
		}
	}

	for cfg.Model == "" {
		fmt.Print("Model Name: ")
		if !scanner.Scan() {
			return "", cfg, types.New(types.ErrCodeInternal, "failed to read model")
		}
		cfg.Model = strings.TrimSpace(scanner.Text())
		if cfg.Model == "" {
			fmt.Println("Error: Model name cannot be empty.")
		}
	}

	for cfg.BaseURL == "" {
		fmt.Print("Base URL (e.g., http://localhost:11434/v1): ")
		if !scanner.Scan() {
			return "", cfg, types.New(types.ErrCodeInternal, "failed to read base url")
		}
		cfg.BaseURL = strings.TrimSpace(scanner.Text())
		if cfg.BaseURL == "" {
			fmt.Println("Error: Base URL is required for custom providers.")
		} else if !strings.HasPrefix(cfg.BaseURL, "http") {
			fmt.Println("Warning: Base URL usually starts with http:// or https://")
		}
	}

	return name, cfg, nil
}
