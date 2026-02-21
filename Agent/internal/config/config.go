package config

import (
	types "agent/internal/Types"
	"strings"

	"github.com/spf13/viper"
)

// LLMConfig holds configuration for a specific LLM provider.
type LLMConfig struct {
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
}

// Config represents the entire application configuration.
type Config struct {
	Environment string               `mapstructure:"env"`
	LLMs        map[string]LLMConfig `mapstructure:"llm"`
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

	if err := viper.ReadInConfig(); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to read config file")
	}

	var conf Config
	if err := viper.Unmarshal(&conf); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to unmarshal config")
	}

	return &conf, nil
}
