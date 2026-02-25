package config

// LLMConfig holds configuration for a specific LLM provider.
// Used by the legacy setup system. New code should use Provider in duckops_config.go.
type LLMConfig struct {
	APIKey  string `mapstructure:"api_key" json:"api_key,omitempty"`
	Model   string `mapstructure:"model" json:"model,omitempty"`
	BaseURL string `mapstructure:"base_url" json:"base_url,omitempty"`
}

// Config is the legacy configuration structure used by SetupService.
// Deprecated: Use DuckOpsConfig (config.toml) for all new config needs.
type Config struct {
	Environment string               `mapstructure:"env" json:"env,omitempty"`
	LLMs        map[string]LLMConfig `mapstructure:"llm" json:"llm,omitempty"`
	Provider    string               `json:"provider"`
}
