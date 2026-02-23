package config

import (
	_ "embed"
)

//go:embed config.default.yaml
var DefaultConfig []byte

// RabbitMQConfig holds configuration for RabbitMQ.
type RabbitMQConfig struct {
	URL      string `mapstructure:"url"`
	Exchange string `mapstructure:"exchange"`
}

// LLMConfig holds configuration for a specific LLM provider.
type LLMConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
}

// Config represents the entire application configuration.
type Config struct {
	Environment string               `mapstructure:"env" json:"env,omitempty"`
	RabbitMQ    RabbitMQConfig       `mapstructure:"rabbitmq" json:"rabbitmq,omitempty"`
	LLMs        map[string]LLMConfig `mapstructure:"llm" json:"llm,omitempty"`
	Provider    string               `json:"provider"`
}
