package main

import (
	"duckops/internal/adapters/cli"
	"duckops/internal/adapters/llm"
	"duckops/internal/adapters/setup"
	"duckops/internal/adapters/tools/chat"
	"duckops/internal/config"
	"duckops/internal/kernel"
	"context"
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

	// ---------- OpenAI ----------
	openaiKey := ""
	openaiModel := "gpt-4o"

	if c, ok := cfg.LLMs["openai"]; ok {
		if c.APIKey != "" {
			openaiKey = c.APIKey
		}
		if c.Model != "" {
			openaiModel = c.Model
		}
	}

	if openaiKey != "" {
		llmRegistry.Register(llm.NewOpenAIAdapter(openaiKey, openaiModel))
	}

	// ---------- Gemini ----------
	geminiKey := ""
	geminiModel := "gemini-1.5-flash"

	if c, ok := cfg.LLMs["gemini"]; ok {
		if c.APIKey != "" {
			geminiKey = c.APIKey
		}
		if c.Model != "" {
			geminiModel = c.Model
		}
	}

	if geminiKey != "" {
		geminiAdapter, err := llm.NewGeminiAdapter(
			context.Background(),
			geminiKey,
			geminiModel,
		)
		if err != nil {
			log.Printf("Warning: Gemini init failed: %v", err)
		} else {
			llmRegistry.Register(geminiAdapter)
			// Ideally defer geminiAdapter.Close() should be handled in a proper cleanup phase
		}
	}

	// ---------- OpenRouter ----------
	orKey := ""
	orModel := "google/gemini-2.5-flash"

	if c, ok := cfg.LLMs["openrouter"]; ok {
		if c.APIKey != "" {
			orKey = c.APIKey
		}
		if c.Model != "" {
			orModel = c.Model
		}
	}

	if orKey != "" {
		llmRegistry.Register(
			llm.NewOpenRouterAdapter(orKey, orModel),
		)
	}

	// ---------- LM Studio ----------
	lmKey := ""
	lmModel := "local-model"
	lmBaseURL := "http://localhost:1234/v1"

	if c, ok := cfg.LLMs["lmstudio"]; ok {
		if c.APIKey != "" {
			lmKey = c.APIKey
		}
		if c.Model != "" {
			lmModel = c.Model
		}
		if c.BaseURL != "" {
			lmBaseURL = c.BaseURL
		}
	}

	// For LM Studio, we always register it as it's a local service to try out and doesn't require keys usually
	llmRegistry.Register(
		llm.NewLMStudioAdapter(lmKey, lmModel, lmBaseURL),
	)

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

	fmt.Println("Agent Kernel initialized successfully.")

	// =========================================================
	// 5. First Run Setup (Provider)
	// =========================================================
	configPath := ".duckops/config.json"
	setupRepo := setup.NewFileSetupRepository(configPath)
	setupPrompter := cli.NewSetupPrompter()

	setupSvc := kernel.NewSetupService(setupRepo, setupPrompter)
	providers := deps.LLM.List()

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
