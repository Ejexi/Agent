package config

import (
	"duckops/internal/types"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// LLMConfig holds configuration for a specific LLM provider.
type LLMConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
}

// Config represents the entire application configuration.
type Config struct {
	Environment string               `mapstructure:"env" json:"env,omitempty"`
	LLMs        map[string]LLMConfig `mapstructure:"llm" json:"llm,omitempty"`
	Provider    string               `json:"provider"`
}

// LoadConfig reads the configuration and expands environment variables.
func LoadConfig(path string) (*Config, error) {
	// 1. Load .env file automatically if it exists (Atomic operation)
	if err := godotenv.Load(); err != nil {
		// Log as info, since .env is optional in production environments (using system env)
	}

	viper.SetConfigFile(path)
	viper.AutomaticEnv() // Merge with system environment

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, types.Wrap(err, types.ErrCodeInternal, "failed to read config file")
		}
	}

	var conf Config
	if err := viper.Unmarshal(&conf); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to unmarshal config")
	}

	// 2. Perform Recursive Expansion on all LLM configurations
	for provider, llmConf := range conf.LLMs {
		llmConf.APIKey = expandValue(provider, "api_key", llmConf.APIKey, true)
		llmConf.Model = expandValue(provider, "model", llmConf.Model, false)
		llmConf.BaseURL = expandValue(provider, "base_url", llmConf.BaseURL, false)

		conf.LLMs[provider] = llmConf
	}

	return &conf, nil
}

// expandValue resolves ${VAR} placeholders and performs sanitization.
func expandValue(provider, field, rawValue string, isSecret bool) string {
	if !strings.Contains(rawValue, "$") {
		return rawValue
	}

	// Standardize syntax: convert $(VAR) to ${VAR} if exists
	expanded := os.ExpandEnv(rawValue)

	// Clean up potential artifacts (quotes, spaces)
	expanded = strings.TrimSpace(expanded)
	expanded = strings.Trim(expanded, `"'`)

	// Production Validation: Check if expansion failed
	if expanded == "" && rawValue != "" {
		log.Printf("[WARNING] Config expansion for %s.%s resulted in an empty string. "+
			"Verify that the environment variable is set in .env or system environment.",
			provider, field)
	}

	return expanded
}

// SaveConfig writes the current configuration back to the file
func SaveConfig(path string, conf *Config) error {
	viper.SetConfigFile(path)

	viper.Set("env", conf.Environment)
	if conf.Provider != "" {
		viper.Set("provider", conf.Provider)
	}

	for provName, llmConf := range conf.LLMs {
		viper.Set(fmt.Sprintf("llm.%s.api_key", provName), llmConf.APIKey)
		viper.Set(fmt.Sprintf("llm.%s.model", provName), llmConf.Model)
		if llmConf.BaseURL != "" {
			viper.Set(fmt.Sprintf("llm.%s.base_url", provName), llmConf.BaseURL)
		}
	}

	if err := viper.WriteConfigAs(path); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to write config file")
	}
	return nil
}
