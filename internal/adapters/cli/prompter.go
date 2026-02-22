package cli

import (
	"duckops/internal/types"
	"bufio"
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
