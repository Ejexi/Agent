package bootstrap

import (
	"context"
	"log"

	"github.com/SecDuckOps/agent/internal/adapters/configsync"
	"github.com/SecDuckOps/agent/internal/adapters/secrets"
	sa "github.com/SecDuckOps/agent/internal/adapters/subagent"
	"github.com/SecDuckOps/agent/internal/config"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/tools/implementations/chat"
	"github.com/SecDuckOps/agent/internal/tools/implementations/delegate"
	"github.com/SecDuckOps/agent/internal/tools/implementations/echo"
	"github.com/SecDuckOps/agent/internal/tools/implementations/scan"
	subagent_tool "github.com/SecDuckOps/agent/internal/tools/implementations/subagent"

	"time"

	"github.com/SecDuckOps/shared/llm/application"
	llm_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/llm/infrastructure"
	"github.com/SecDuckOps/shared/logger"
	shared_ports "github.com/SecDuckOps/shared/ports"
)

// App holds the initialized application components.
type App struct {
	Kernel   *kernel.Kernel
	Sessions ports.SessionManager
	Provider string
	Logger   shared_ports.Logger
}

// FromTOML bootstraps the application from ~/.duckops/config.toml.
func FromTOML(tomlCfg *config.DuckOpsConfig) *App {
	appLogger, err := logger.New("duckops-agent", "info")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	ctx := context.Background()

	profile, ok := tomlCfg.GetProfile("default")
	if !ok {
		appLogger.ErrorErr(ctx, "agent_config_failed", nil, "No 'default' profile found in config.toml")
		log.Fatal("No 'default' profile found in config.toml")
	}

	// Capability Registry holds injected subagent profiles
	capabilityRegistry := sa.NewCapabilityRegistry()

	// ---------------------------------------------------------
	// SUPER MODE LOGIC
	// ---------------------------------------------------------
	if tomlCfg.Settings.AgentMode == "super" {
		appLogger.Info(ctx, "config_sync_started", "⚡ Starting in SUPER MODE. Connecting to API Gateway...", shared_ports.Field{Key: "url", Value: tomlCfg.Settings.APIGatewayURL})
		syncAdapter := configsync.NewHTTPAdapter(tomlCfg.Settings.APIGatewayURL, "") // TODO: API Key

		remoteCfg, err := syncAdapter.FetchRemoteConfig(ctx)
		if err != nil {
			appLogger.ErrorErr(ctx, "config_sync_failed", err, "Failed to fetch remote config on startup, falling back to local config")
		} else {
			appLogger.Info(ctx, "config_sync_success", "Successfully fetched remote config", shared_ports.Field{Key: "rules_count", Value: len(remoteCfg.Rules)})
			capabilityRegistry.Sync(remoteCfg.Capabilities)
			// TODO: Merge remoteCfg into local profile
		}

		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					cfg, err := syncAdapter.FetchRemoteConfig(context.Background())
					if err != nil {
						appLogger.ErrorErr(context.Background(), "config_sync_failed", err, "Periodic config sync failed")
					} else {
						capabilityRegistry.Sync(cfg.Capabilities)
						// silent success, no need to spam logs unless changed
						// TODO: Apply updates to running kernel dynamically
					}
				}
			}
		}()
	}
	// ---------------------------------------------------------

	llmRegistry := buildLLMRegistry(profile, appLogger)

	// Kernel
	deps := kernel.Dependencies{
		LLM:    llmRegistry,
		Logger: appLogger,
	}
	k := kernel.New(deps)
	if k == nil {
		appLogger.ErrorErr(ctx, "kernel_init_failed", nil, "Kernel initialization failed")
		log.Fatal("Kernel initialization failed")
	}

	// Tracker (implements ports.SessionManager)
	bridge := &sa.KernelBridge{
		ExecuteFn:    k.Execute,
		GetSchemasFn: k.GetToolSchemas,
		LLMRegistry:  llmRegistry,
	}

	// Initialize Secret Scanner
	var secretScanner ports.SecretScannerPort
	if profile.Secrets != nil && profile.Secrets.Enabled {
		// Use empty string to load just the default embedded patterns
		scanner, scanErr := secrets.NewWithCustomPatterns("")
		if scanErr != nil {
			appLogger.ErrorErr(ctx, "security_scanner_failed", scanErr, "Failed to initialize Secret Scanner, secrets will NOT be scrubbed")
		} else {
			secretScanner = scanner
			appLogger.Info(ctx, "security_scanner_initialized", "Secret Scanner initialized successfully")
		}
	}

	tracker := sa.NewTracker(bridge, bridge, secretScanner, appLogger)

	// Register tools
	registerTools(k, deps, tracker, capabilityRegistry, appLogger)

	provider := profile.Provider
	if provider == "" {
		providers := llmRegistry.List()
		if len(providers) > 0 {
			provider = providers[0]
		}
	}

	appLogger.Info(ctx, "agent_start", "🦆 DuckOps Agent initialized successfully")
	return &App{
		Kernel:   k,
		Sessions: tracker,
		Provider: provider,
		Logger:   appLogger,
	}
}

// buildLLMRegistry bridges TOML providers → shared LLM registry.
func buildLLMRegistry(profile config.Profile, appLogger shared_ports.Logger) llm_domain.LLMRegistry {
	sharedCfg := llm_domain.Config{
		Default:   profile.Provider,
		Providers: make(map[string]llm_domain.ProviderConfig),
	}
	for name, prov := range profile.Providers {
		sharedCfg.Providers[name] = llm_domain.ProviderConfig{
			APIKey:  prov.APIKey,
			Model:   prov.Model,
			BaseURL: prov.BaseURL,
		}
	}

	llmRegistry, err := application.NewLLMRegistry(sharedCfg)
	if err != nil {
		appLogger.ErrorErr(context.Background(), "agent_start", err, "Failed to initialize LLM Registry")
		log.Fatalf("Failed to initialize LLM Registry: %v", err)
	}

	// Gemini special handling (uses native SDK, not OpenAI-compatible)
	if prov, ok := profile.Providers["gemini"]; ok && prov.APIKey != "" {
		geminiAdapter, err := infrastructure.NewGeminiAdapter(
			context.Background(), prov.APIKey, prov.Model,
		)
		if err != nil {
			appLogger.ErrorErr(context.Background(), "agent_start", err, "Warning: Gemini init failed")
		} else {
			llmRegistry.Register(geminiAdapter)
		}
	}

	return llmRegistry
}

// registerTools registers all agent tools with the kernel.
func registerTools(k *kernel.Kernel, deps kernel.Dependencies, tracker *sa.Tracker, registry *sa.CapabilityRegistry, appLogger shared_ports.Logger) {
	tools := []struct {
		name string
		err  error
	}{
		{"chat", k.RegisterTool(chat.NewChatTool(deps.LLM))},
		{"echo", k.RegisterTool(echo.NewEchoTool())},
		{"scan", k.RegisterTool(scan.NewScanTool(deps.LLM, deps.Memory))},
		{"subagent", k.RegisterTool(subagent_tool.NewSubagentTool(tracker))},
		{"resume", k.RegisterTool(subagent_tool.NewResumeTool(tracker))},
		{"delegate", k.RegisterTool(delegate.NewDelegateTool(tracker, registry))},
	}
	for _, t := range tools {
		if t.err != nil {
			appLogger.ErrorErr(context.Background(), "operation_failed", t.err, "Tool registration failed", shared_ports.Field{Key: "tool", Value: t.name})
			log.Fatalf("%s tool registration failed: %v", t.name, t.err)
		}
	}
}
