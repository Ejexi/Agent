package config

import (
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	App AppConfig
	// Database DatabaseConfig
	LLM LLMConfig
}

// AppConfig holds application-level settings
type AppConfig struct {
	Env      string
	LogLevel string `mapstructure:"log_level"`
}

// DatabaseConfig holds database connection settings
// type DatabaseConfig struct {
// 	Host     string
// 	Port     int
// 	Database string
// 	User     string
// 	Password string
// }

// llm config
type LLMConfig struct {
	APIKey      string
	BaseURL     string `mapstructure:"base_url"`
	Model       string
	Temperature float64
	MaxTokens   int `mapstructure:"max_tokens"`
	Timeout     time.Duration
}

// Take config from yaml file
func Load(configPath string) (*Config, error) {
	//read settings
	viper.SetConfigFile(configPath)
	//env var reading
	viper.AutomaticEnv()

	//set default values
	viper.SetDefault("app.env", "development")
	viper.SetDefault("app.log_level", "info")
	viper.SetDefault("llm.timeout", "30s")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	//JSON/YAML Create a Config struct
	var config Config

	// Fill the Config struct with values from the file
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Get API key from environment
	if apiKey := os.Getenv("API_KEY_AI"); apiKey != "" {
		config.LLM.APIKey = apiKey
	}
	return &config, nil
}

//this func will get called in load config or main.go will decide latter and viper
//will call the yaml file config.Load("config.yaml") make to struct file cfg
