package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/SecDuckOps/agent/internal/adapters/configsync"
	"github.com/SecDuckOps/agent/internal/adapters/executor"
	"github.com/SecDuckOps/agent/internal/adapters/secrets"
	"github.com/SecDuckOps/agent/internal/adapters/security"
	sa "github.com/SecDuckOps/agent/internal/adapters/subagent"
	"github.com/SecDuckOps/agent/internal/adapters/translator"
	warden_adapter "github.com/SecDuckOps/agent/internal/adapters/warden"
	"github.com/SecDuckOps/agent/internal/application/taskengine"
	"github.com/SecDuckOps/agent/internal/config"
	domain_security "github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/tools/implementations/chat"
	"github.com/SecDuckOps/agent/internal/tools/implementations/delegate"
	"github.com/SecDuckOps/agent/internal/tools/implementations/scan"
	"github.com/SecDuckOps/agent/internal/tools/implementations/subagent"
	"github.com/SecDuckOps/agent/internal/tools/implementations/terminal"
	"github.com/google/uuid"

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
	dir, _ := config.DuckOpsDir()
	logPath := filepath.Join(dir, "duckops.log")

	appLogger, err := logger.New("duckops-agent", "info", logPath)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	ctx := context.Background()

	profile, ok := tomlCfg.GetProfile("default")
	if !ok {
		appLogger.ErrorErr(ctx, fmt.Errorf("agent_config_failed"), "No 'default' profile found in config.toml")
		log.Fatal("No 'default' profile found in config.toml")
	}

	// Capability Registry holds injected subagent profiles
	capabilityRegistry := sa.NewCapabilityRegistry()

	// ---------------------------------------------------------
	// Super Duck LOGIC
	// ---------------------------------------------------------
	if tomlCfg.Settings.AgentMode == "super" {
		appLogger.Info(ctx, "⚡ Starting in Super Duck. Connecting to API Gateway...", shared_ports.Field{Key: "url", Value: tomlCfg.Settings.APIGatewayURL})
		syncAdapter := configsync.NewHTTPAdapter(tomlCfg.Settings.APIGatewayURL, "") // TODO: API Key

		remoteCfg, err := syncAdapter.FetchRemoteConfig(ctx)
		if err != nil {
			appLogger.ErrorErr(ctx, err, "Failed to fetch remote config on startup, falling back to local config")
		} else {
			appLogger.Info(ctx, "Successfully fetched remote config", shared_ports.Field{Key: "rules_count", Value: len(remoteCfg.Rules)})
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
						appLogger.ErrorErr(context.Background(), err, "Periodic config sync failed")
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

	// Initialize Warden (Cedar policy evaluator)
	useDefaultDeny := true
	if profile.Warden != nil {
		useDefaultDeny = profile.Warden.DefaultDeny
	}
	wardenInstance := warden_adapter.New(useDefaultDeny)

	// Load policies
	var policies []domain_security.NetworkPolicy
	if profile.Warden != nil && len(profile.Warden.PolicyFiles) > 0 {
		for _, path := range profile.Warden.PolicyFiles {
			content, err := os.ReadFile(path)
			if err != nil {
				appLogger.ErrorErr(ctx, err, "Failed to read policy file", shared_ports.Field{Key: "path", Value: path})
				continue
			}
			policies = append(policies, domain_security.NetworkPolicy{
				ID:        uuid.New().String(),
				Name:      filepath.Base(path),
				CedarBody: string(content),
				Enabled:   true,
				Priority:  10,
			})
		}
	}

	if err := wardenInstance.LoadPolicies(ctx, policies); err != nil {
		appLogger.ErrorErr(ctx, err, "Failed to load Warden policies")
	}

	// Kernel
	deps := kernel.Dependencies{
		LLM:    llmRegistry,
		Logger: appLogger,
		Warden: wardenInstance,
	}
	k := kernel.New(deps)
	if k == nil {
		appLogger.ErrorErr(ctx, fmt.Errorf("kernel_init_failed"), "Kernel initialization failed")
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
			appLogger.ErrorErr(ctx, scanErr, "Failed to initialize Secret Scanner, secrets will NOT be scrubbed")
		} else {
			secretScanner = scanner
			appLogger.Info(ctx, "Secret Scanner initialized successfully")
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

	appLogger.Info(ctx, "🦆 DuckOps Agent initialized successfully")
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
		appLogger.ErrorErr(context.Background(), err, "Failed to initialize LLM Registry")
		log.Fatalf("Failed to initialize LLM Registry: %v", err)
	}

	// Gemini special handling (uses native SDK, not OpenAI-compatible)
	if prov, ok := profile.Providers["gemini"]; ok && prov.APIKey != "" {
		geminiAdapter, err := infrastructure.NewGeminiAdapter(
			context.Background(), prov.APIKey, prov.Model,
		)
		if err != nil {
			appLogger.ErrorErr(context.Background(), err, "Warning: Gemini init failed")
		} else {
			llmRegistry.Register(geminiAdapter)
		}
	}

	return llmRegistry
}

// registerTools registers all agent tools with the kernel.
func registerTools(k *kernel.Kernel, deps kernel.Dependencies, tracker *sa.Tracker, registry *sa.CapabilityRegistry, appLogger shared_ports.Logger) {
	// Setup Hexagonal Task Engine Middleware Pipeline
	osTranslator := translator.NewOSTranslatorAdapter("") // default to current OS
	osExecutor := executor.NewOSExecAdapter(appLogger)
	taskWarden := security.NewTaskWardenAdapter(deps.Warden, osTranslator, appLogger)

	taskDispatcher := taskengine.NewDispatcher(osTranslator, taskWarden, osExecutor, appLogger)

	tools := []struct {
		name string
		err  error
	}{
		{"chat", k.RegisterTool(chat.NewChatTool(deps.LLM))},
		{"scan", k.RegisterTool(scan.NewScanTool(deps.LLM, deps.Memory))},
		{"subagent", k.RegisterTool(subagent.NewSubagentTool(tracker))},
		{"resume", k.RegisterTool(subagent.NewResumeTool(tracker))},
		{"delegate", k.RegisterTool(delegate.NewDelegateTool(tracker, registry))},
		{"terminal", k.RegisterTool(terminal.NewTerminalTool(taskDispatcher))},
	}
	for _, t := range tools {
		if t.err != nil {
			appLogger.ErrorErr(context.Background(), t.err, "Tool registration failed", shared_ports.Field{Key: "tool", Value: t.name})
			log.Fatalf("%s tool registration failed: %v", t.name, t.err)
		}
	}
}
