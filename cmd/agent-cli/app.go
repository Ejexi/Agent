package main

import (
	"context"
	"duckops/internal/adapters/cli"
	"duckops/internal/adapters/llm"
	"duckops/internal/adapters/setup"
	"duckops/internal/config"
	"duckops/internal/kernel"
	"duckops/internal/tools/implementations/chat"
	"duckops/internal/tools/implementations/echo"
	"duckops/internal/tools/implementations/scan"
	"fmt"
	"log"
)

// InitApp orchestrates the Dependency Injection, configuration, and setup process
func InitApp() (*kernel.Kernel, string) {
	// =========================================================
	// 1. Load Configuration
	// =========================================================
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Loaded config for environment: %s\n", cfg.Environment)

	// =========================================================
	// 2. Initialize LLM Registry
	// =========================================================
	llmRegistry := llm.NewRegistryAdapter("openai")

	// 2.1 Register Standard/Static Providers if needed,
	// but now we prioritize the dynamic config.

	// ---------- Gemini (Special Handling for generativeai-go SDK) ----------
	if c, ok := cfg.LLMs["gemini"]; ok && c.APIKey != "" {
		geminiAdapter, err := llm.NewGeminiAdapter(
			context.Background(),
			c.APIKey,
			c.Model,
		)
		if err != nil {
			log.Printf("Warning: Gemini init failed: %v", err)
		} else {
			llmRegistry.Register(geminiAdapter)
		}
	}

	// 2.2 Register all other providers dynamically (including OpenAI, OpenRouter, and Custom)
	// If they have a base_url, they will use the OpenAICompatibleAdapter automatically.
	llmRegistry.RegisterFromConfig(cfg.LLMs)

	// Explicitly handle standard OpenAI if not already handled by dynamic config logic
	// (RegisterFromConfig handles it if it's in the map)

	// =========================================================
	// 3. Build Kernel
	// =========================================================
	deps := kernel.Dependencies{
		LLM: llmRegistry,
	}

	k := kernel.New(deps)
	if k == nil {
		log.Fatal("Kernel initialization failed")
	}

	// =========================================================
	// 4. Register Tools
	// =========================================================
	chatTool := chat.NewChatTool(deps.LLM)
	if err := k.RegisterTool(chatTool); err != nil {
		log.Fatalf("Chat tool registration failed: %v", err)
	}

	echoTool := echo.NewEchoTool()
	if err := k.RegisterTool(echoTool); err != nil {
		log.Fatalf("Echo tool registration failed: %v", err)
	}

	scanTool := scan.NewScanTool(deps.LLM, deps.Memory)
	if err := k.RegisterTool(scanTool); err != nil {
		log.Fatalf("Scan tool registration failed: %v", err)
	}

	fmt.Println("Agent Kernel initialized successfully.")

	// =========================================================
	// 5. First Run Setup (Provider)
	// =========================================================
	configPath := ".duckops/config.json"
	setupRepo := setup.NewFileSetupRepository(configPath)
	setupPrompter := cli.NewSetupPrompter()

	setupSvc := kernel.NewSetupService(setupRepo, setupPrompter)

	// [New] Dynamic Provider Configuration
	if err := setupSvc.ConfigureCustomProvider(cfg); err != nil {
		log.Printf("Warning: Custom provider configuration failed: %v", err)
	}

	// Re-sync registry in case a new provider was added during configuration
	llmRegistry.RegisterFromConfig(cfg.LLMs)

	providers := llmRegistry.List()
	selectedProvider, err := setupSvc.GetProvider(providers)
	if err != nil {
		log.Fatalf("Provider setup failed: %v", err)
	}

	// fallback safety
	if selectedProvider == "" && len(providers) > 0 {
		selectedProvider = providers[0]
	}

	return k, selectedProvider
}
