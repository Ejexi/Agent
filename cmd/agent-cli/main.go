package main

import (
	"agent/internal/adapters/llm"
	"agent/internal/config"
	"agent/internal/domain"
	"agent/internal/kernel"
	"agent/internal/tools/scan"
	"context"
	"fmt"
	"log"
)

// DummyBus implements ports.BusPort for demonstration.
type DummyBus struct{}

func (b *DummyBus) Publish(ctx context.Context, topic string, message interface{}) error { return nil }
func (b *DummyBus) Subscribe(ctx context.Context, topic string, handler func(domain.Task)) error {
	return nil
}

func main() {
	// ============================================
	// 1. Load Configuration (YAML via Viper)
	// ============================================
	conf, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	fmt.Printf("Loaded config for environment: %s\n", conf.Environment)

	// ============================================
	// 2. Initialize Infrastructure / Adapters
	// ============================================
	bus := &DummyBus{}

	// Create our brand new LLM Multiplexer Registry
	llmRegistry := llm.NewRegistryAdapter("openai")

	// Inject OpenAI if configured
	if openaiConf, ok := conf.LLMs["openai"]; ok && openaiConf.APIKey != "" {
		llmRegistry.Register(llm.NewOpenAIAdapter(openaiConf.APIKey, openaiConf.Model))
	}

	// Inject Gemini if configured
	if geminiConf, ok := conf.LLMs["gemini"]; ok && geminiConf.APIKey != "" {
		geminiAdapter, err := llm.NewGeminiAdapter(context.Background(), geminiConf.APIKey, geminiConf.Model)
		if err == nil {
			llmRegistry.Register(geminiAdapter)
			defer geminiAdapter.Close() // Graceful cleanup at end of execution
		} else {
			log.Printf("Warning: failed to initialize gemini adapter: %v", err)
		}
	}

	// Inject OpenRouter if configured
	if orConf, ok := conf.LLMs["openrouter"]; ok && orConf.APIKey != "" {
		llmRegistry.Register(llm.NewOpenRouterAdapter(orConf.APIKey, orConf.Model))
	}

	// ============================================
	// 3. Assemble Agent Core Kernel
	// ============================================
	deps := kernel.Dependencies{
		MessageBus: bus,
		LLM:        llmRegistry,
		Memory:     nil, // We will build PostgresAdapter later
	}
	k := kernel.New(deps)
	if k == nil {
		log.Fatal("Failed to initialize Agent Kernel")
	}

	// ============================================
	// 4. Initialize Tools (Dependency Injection)
	// ============================================
	// Notice: The tool does not care about Viper or Config.
	// The tool only cares that it received a ready logic `LLMRegistry`.
	scanTool := scan.NewScanTool(deps.LLM, deps.Memory)

	if err := k.RegisterTool(scanTool); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}

	fmt.Println("Agent Kernel initialized and ready.")

	// ============================================
	// 5. Test Execution using dynamic LLM selection
	// ============================================
	task := domain.Task{
		ID:   "1",
		Tool: "scan",
		Args: map[string]interface{}{
			"target":      "duckduckgo.com",
			"ai_provider": "gemini", // <== This tells the Registry to dynamically route the request!
		},
	}

	res, err := k.Execute(context.Background(), task)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Execution Result: %+v\n", res)
}
