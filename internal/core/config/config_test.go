package config

import "testing"

func TestLoad(t *testing.T) {
	cfg, err := Load("../../../configs/dev/config.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg.App.Env != "development" {
		t.Errorf("Expected env=development, got %s", cfg.App.Env)
	}

	// if cfg.Database.Host != "localhost" {
	// 	t.Errorf("Expected database host=localhost, got %s", cfg.Database.Host)
	// }

	if cfg.LLM.Model == "" {
		t.Error("LLM model should not be empty")
	}
}
