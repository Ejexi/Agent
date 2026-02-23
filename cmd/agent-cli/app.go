package main

import (
	"context"
	"fmt"
	"log"

	"github.com/SecDuckOps/agent/internal/adapters/cli"
	"github.com/SecDuckOps/agent/internal/adapters/setup"
	"github.com/SecDuckOps/agent/internal/config"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/agent/internal/tools/implementations/chat"
	"github.com/SecDuckOps/agent/internal/tools/implementations/echo"
	"github.com/SecDuckOps/agent/internal/tools/implementations/scan"

	"github.com/SecDuckOps/shared/llm/application"
	"github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/llm/infrastructure"
)

// InitApp orchestrates the Dependency Injection, configuration, and setup process
func InitApp() (*kernel.Kernel, string) {
	// =========================================================
	// 1. Load Configuration
	// =========================================================
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Loaded config for environment: %s\n", cfg.Environment)

	// =========================================================
	// 2. Initialize LLM Registry
	// =========================================================
	sharedCfg := domain.Config{
		Default:   "openai",
		Providers: make(map[string]domain.ProviderConfig),
	}
	for name, c := range cfg.LLMs {
		sharedCfg.Providers[name] = domain.ProviderConfig{
			APIKey:  c.APIKey,
			Model:   c.Model,
			BaseURL: c.BaseURL,
		}
	}

	llmRegistry, err := application.NewLLMRegistry(sharedCfg)
	if err != nil {
		log.Fatalf("Failed to initialize LLM Registry: %v", err)
	}

	// ---------- Gemini (Special Handling for generativeai-go SDK) ----------
	if c, ok := cfg.LLMs["gemini"]; ok && c.APIKey != "" {
		geminiAdapter, err := infrastructure.NewGeminiAdapter(
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
	newSharedCfg := make(map[string]domain.ProviderConfig)
	for name, c := range cfg.LLMs {
		newSharedCfg[name] = domain.ProviderConfig{
			APIKey:  c.APIKey,
			Model:   c.Model,
			BaseURL: c.BaseURL,
		}
	}
	llmRegistry.RegisterFromConfig(newSharedCfg)

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
