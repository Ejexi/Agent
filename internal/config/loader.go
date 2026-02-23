package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/SecDuckOps/Shared/types"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Load reads the configuration and expands environment variables.
// Priority:
// 1. Environment variables
// 2. External config.yaml (if exists)
// 3. Embedded config.default.yaml (fallback)
func Load() (*Config, error) {
	// Load .env file automatically if it exists
	if err := godotenv.Load(); err != nil {
		// Log as info, since .env is optional
	}

	v := viper.New()

	// Set configuration type for the embedded content
	v.SetConfigType("yaml")

	// 1. Load embedded defaults
	if err := v.ReadConfig(bytes.NewReader(DefaultConfig)); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to read embedded config")
	}

	// 2. Try to load external config.yaml
	v.SetConfigFile("config.yaml")
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// If file exists but is invalid, return error
			if !os.IsNotExist(err) {
				return nil, types.Wrap(err, types.ErrCodeInternal, "failed to read external config file")
			}
		}
	}

	// 3. Automatic Environment Variables (DUCKOPS_*)
	v.SetEnvPrefix("DUCKOPS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var conf Config
	if err := v.Unmarshal(&conf); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to unmarshal config")
	}

	// Perform Recursive Expansion on all LLM configurations
	for provider, llmConf := range conf.LLMs {
		llmConf.APIKey = ExpandValue(provider, "api_key", llmConf.APIKey)
		llmConf.Model = ExpandValue(provider, "model", llmConf.Model)
		llmConf.BaseURL = ExpandValue(provider, "base_url", llmConf.BaseURL)

		conf.LLMs[provider] = llmConf
	}

	// Expand RabbitMQ URL if it contains environment variables
	conf.RabbitMQ.URL = ExpandValue("rabbitmq", "url", conf.RabbitMQ.URL)

	return &conf, nil
}

// ExpandValue resolves ${VAR} placeholders and performs sanitization.
func ExpandValue(provider, field, rawValue string) string {
	if !strings.Contains(rawValue, "$") {
		return rawValue
	}

	expanded := os.ExpandEnv(rawValue)

	expanded = strings.TrimSpace(expanded)
	expanded = strings.Trim(expanded, `"'`)

	if expanded == "" && rawValue != "" {
		log.Printf("[WARNING] Config expansion for %s.%s resulted in an empty string. "+
			"Verify that the environment variable is set in .env or system environment.",
			provider, field)
	}

	return expanded
}

// SaveConfig writes the current configuration back to a file
func SaveConfig(path string, conf *Config) error {
	v := viper.New()
	v.SetConfigFile(path)

	v.Set("env", conf.Environment)
	v.Set("provider", conf.Provider)
	v.Set("rabbitmq.url", conf.RabbitMQ.URL)
	v.Set("rabbitmq.exchange", conf.RabbitMQ.Exchange)

	for provName, llmConf := range conf.LLMs {
		v.Set(fmt.Sprintf("llm.%s.api_key", provName), llmConf.APIKey)
		v.Set(fmt.Sprintf("llm.%s.model", provName), llmConf.Model)
		if llmConf.BaseURL != "" {
			v.Set(fmt.Sprintf("llm.%s.base_url", provName), llmConf.BaseURL)
		}
	}

	if err := v.WriteConfigAs(path); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to write config file")
	}
	return nil
}
