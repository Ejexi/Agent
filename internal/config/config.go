package config

import (
	types "duckops/internal/types"
	"fmt"
	"os"
	"strings"

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

// LoadConfig reads the configuration from a file and overrides it via OS environment variables.
func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)

	// Enable reading from OS Environment Variabels
	// Prefix will be "AGENT_"
	viper.SetEnvPrefix("AGENT")

	// Map dots in config keys to underscores in env vars (e.g. llm.openai.api_key -> AGENT_LLM_OPENAI_API_KEY)
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	// Explicitly bind standard OS env vars to config so you don't need 'AGENT_' prefix for API keys
	viper.BindEnv("llm.openai.api_key", "OPENAI_API_KEY")
	viper.BindEnv("llm.gemini.api_key", "GEMINI_API_KEY")
	viper.BindEnv("llm.openrouter.api_key", "OPENROUTER_API_KEY")
	viper.BindEnv("llm.lmstudio.api_key", "LMSTUDIO_API_KEY")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, types.Wrap(err, types.ErrCodeInternal, "failed to read config file")
		}
		// If file doesn't exist, we just return an empty config to be populated by setup
	}

	var conf Config
	if err := viper.Unmarshal(&conf); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to unmarshal config")
	}

	// Expand environment variables found within the config values (e.g., $(OPENROUTER_API_KEY) or ${OPENROUTER_API_KEY})
	for provider, llmConf := range conf.LLMs {
		// Convert $(VAR) to ${VAR} for os.ExpandEnv compatibility
		expandedKey := strings.ReplaceAll(llmConf.APIKey, "$(", "${")
		// Safely handle closing bracket if present for $() syntax
		if strings.Contains(llmConf.APIKey, "$(") {
			expandedKey = strings.ReplaceAll(expandedKey, ")", "}")
		}

		val := os.ExpandEnv(expandedKey)
		// Clean up surrounding quotes and spaces that Windows users might accidentally include via CMD
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)

		llmConf.APIKey = val
		conf.LLMs[provider] = llmConf
	}

	return &conf, nil
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
