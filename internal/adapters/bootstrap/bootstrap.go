package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/SecDuckOps/agent/internal/adapters/configsync"
	"github.com/SecDuckOps/agent/internal/adapters/events"
	"github.com/SecDuckOps/agent/internal/adapters/executor"
	"github.com/SecDuckOps/agent/internal/adapters/secrets"
	"github.com/SecDuckOps/agent/internal/adapters/security"
	agent_app "github.com/SecDuckOps/agent/internal/application"
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
	"github.com/SecDuckOps/agent/internal/tools/implementations/file_ops"
	"github.com/SecDuckOps/agent/internal/tools/implementations/notes"
	"github.com/SecDuckOps/agent/internal/tools/implementations/reporting"
	"github.com/SecDuckOps/agent/internal/tools/implementations/scan"
	"github.com/SecDuckOps/agent/internal/tools/implementations/skills"
	"github.com/SecDuckOps/agent/internal/tools/implementations/subagent"
	"github.com/SecDuckOps/agent/internal/tools/implementations/terminal"
	"github.com/SecDuckOps/agent/internal/tools/implementations/todo"
	domain_skills "github.com/SecDuckOps/agent/internal/skills"
	"github.com/google/uuid"

	"time"

	"github.com/SecDuckOps/shared/llm/application"
	llm_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/llm/infrastructure"
	"github.com/SecDuckOps/shared/logger"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/scanner/aggregator"
	"github.com/SecDuckOps/shared/scanner/parsers/container/grype"
	"github.com/SecDuckOps/shared/scanner/parsers/container/trivy"
	"github.com/SecDuckOps/shared/scanner/parsers/dast/nuclei"
	"github.com/SecDuckOps/shared/scanner/parsers/dast/zap"
	"github.com/SecDuckOps/shared/scanner/parsers/dependency/dependencycheck"
	"github.com/SecDuckOps/shared/scanner/parsers/dependency/osvscanner"
	"github.com/SecDuckOps/shared/scanner/parsers/iac/checkov"
	"github.com/SecDuckOps/shared/scanner/parsers/iac/kics"
	"github.com/SecDuckOps/shared/scanner/parsers/iac/terrascan"
	"github.com/SecDuckOps/shared/scanner/parsers/iac/tflint"
	"github.com/SecDuckOps/shared/scanner/parsers/iac/tfsec"
	"github.com/SecDuckOps/shared/scanner/parsers/sast/bandit"
	"github.com/SecDuckOps/shared/scanner/parsers/sast/brakeman"
	"github.com/SecDuckOps/shared/scanner/parsers/sast/gosec"
	"github.com/SecDuckOps/shared/scanner/parsers/sast/njsscan"
	"github.com/SecDuckOps/shared/scanner/parsers/sast/semgrep"
	"github.com/SecDuckOps/shared/scanner/parsers/secrets/detectsecrets"
	"github.com/SecDuckOps/shared/scanner/parsers/secrets/gitleaks"
	"github.com/SecDuckOps/shared/scanner/parsers/secrets/trufflehog"
	scanner_ports "github.com/SecDuckOps/shared/scanner/ports"
)

// App holds the initialized application components.
type App struct {
	Kernel       *kernel.Kernel
	Sessions     ports.SessionManager    // Subagent tracker
	AppSessions  ports.AppSessionManager // Main workspace sessions
	Provider     string
	Model        string
	Logger       shared_ports.Logger
	EventBus     ports.EventBusPort
	SkillRegistry domain_skills.Registry
	Shutdown     func()
}

// FromTOML bootstraps the application from ~/.duckops/config.toml.
func FromTOML(parentCtx context.Context, tomlCfg *config.DuckOpsConfig) *App {
	ctx, cancel := context.WithCancel(parentCtx)
	dir, _ := config.DuckOpsDir()
	logPath := filepath.Join(dir, "duckops.log")

	appLogger, err := logger.New("duckops-agent", "info", logPath)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Initialize the EventBus
	eventBus := events.NewInMemoryEventBus(appLogger)

	// Initialize App Session Manager (Phase 1 Enhancements)
	appSessionManager := agent_app.NewSessionManagerService(appLogger, eventBus)
	
	profile, ok := tomlCfg.GetProfile("default")
	if !ok {
		appLogger.ErrorErr(ctx, fmt.Errorf("agent_config_failed"), "No 'default' profile found in config.toml")
		log.Fatal("No 'default' profile found in config.toml")
	}

	// Create initial Agent workspace session
	cwd, _ := os.Getwd()
	_, err = appSessionManager.CreateSession(ctx, cwd, tomlCfg.Settings.AgentMode, profile.Model)
	if err != nil {
		appLogger.ErrorErr(ctx, err, "Failed to create root agent session")
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
				case <-ctx.Done():
					return
				case <-ticker.C:
					cfg, err := syncAdapter.FetchRemoteConfig(ctx)
					if err != nil {
						appLogger.ErrorErr(ctx, err, "Periodic config sync failed")
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
	wardenInstance := warden_adapter.New(useDefaultDeny, appLogger)

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

	// Setup OS adapters for the kernel and dispatcher
	osExecutor := executor.NewOSExecAdapter(appLogger)

	// Tools (Phase 2 Enhancements)
	toolRegistry := agent_app.NewToolRegistryService(appLogger)

	// Kernel
	deps := kernel.Dependencies{
		ToolRegistry:   toolRegistry,
		LLM:            llmRegistry,
		Logger:         appLogger,
		Warden:         wardenInstance,
		ShellExecution: osExecutor,
		ShellLifecycle: osExecutor,
	}
	k := kernel.New(deps)
	if k == nil {
		appLogger.ErrorErr(ctx, fmt.Errorf("kernel_init_failed"), "Kernel initialization failed")
		log.Fatal("Kernel initialization failed")
	}

	// Tracker (implements ports.SessionManager)
	bridge := &sa.KernelBridge{
		ExecuteFn:    k.ExecuteCompat,
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

	// Initialize Docker Warden (Scanner Port)
	var dockerWarden *warden_adapter.DockerWarden
	
	dw, err := warden_adapter.NewDockerWarden()
	if err != nil {
		appLogger.ErrorErr(ctx, err, "Failed to initialize Docker Warden client. Container scans will not be available.")
	} else {
		// Verify daemon connection
		if err := dw.HealthCheck(ctx); err != nil {
			appLogger.ErrorErr(ctx, err, "Docker Warden initial healthcheck failed. Trying to start Docker automatically...")
			
			// Attempt to auto-start!
			autoStartErr := warden_adapter.AutoStartDocker(ctx, appLogger, dw)
			if autoStartErr != nil {
				appLogger.ErrorErr(ctx, autoStartErr, "Failed to auto-start Docker. Container scans will not be available.")
			} else {
				dockerWarden = dw
				appLogger.Info(ctx, "Docker Warden initialized successfully after auto-start")
			}
		} else {
			dockerWarden = dw
			appLogger.Info(ctx, "Docker Warden initialized successfully")
		}
	}
	
	// Initialize the Scanner Aggregator with all registered parsers
	var scannerSvc *aggregator.ScannerService
	if dockerWarden != nil {
		parsers := []scanner_ports.ResultParserPort{
			trivy.NewTrivyParser(),
			semgrep.NewSemgrepParser(),
			gitleaks.NewGitleaksParser(),
			zap.NewZapParser(),
			tfsec.NewTfsecParser(),
			dependencycheck.NewDependencyCheckParser(),
			gosec.NewGosecParser(),
			bandit.NewBanditParser(),
			njsscan.NewNjsscanParser(),
			checkov.NewCheckovParser(),
			trufflehog.NewTrufflehogParser(),
			brakeman.NewBrakemanParser(),
			kics.NewKicsParser(),
			grype.NewGrypeParser(),
			nuclei.NewNucleiParser(),
			osvscanner.NewOsvScannerParser(),
			terrascan.NewTerrascanParser(),
			tflint.NewTflintParser(),
			detectsecrets.NewDetectSecretsParser(),
		}
		scannerSvc = aggregator.NewScannerService(dockerWarden, parsers)
	}

	// Register tools
	skillRegistry := registerTools(ctx, toolRegistry, deps, tracker, bridge, capabilityRegistry, profile, appLogger, scannerSvc)

	provider := profile.Provider
	if provider == "" {
		providers := llmRegistry.List()
		if len(providers) > 0 {
			provider = providers[0]
		}
	}

	appLogger.Info(ctx, "🦆 DuckOps Agent initialized successfully")
	return &App{
		Kernel:        k,
		Sessions:      tracker,
		AppSessions:   appSessionManager,
		Provider:      provider,
		Model:         profile.Model,
		Logger:        appLogger,
		EventBus:      eventBus,
		SkillRegistry: skillRegistry,
		Shutdown:      cancel,
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
	for name, prov := range profile.Providers {
		if (name == "gemini" || prov.Type == "gemini") && prov.APIKey != "" {
			geminiAdapter, err := infrastructure.NewGeminiAdapter(
				context.Background(), prov.APIKey, prov.Model,
			)
			if err != nil {
				appLogger.ErrorErr(context.Background(), err, "Warning: Gemini init failed for "+name)
			} else {
				llmRegistry.Register(geminiAdapter)
			}
		}
	}

	return llmRegistry
}

// registerTools registers all agent tools with the kernel.
func registerTools(ctx context.Context, toolRegistry ports.ToolRegistry, deps kernel.Dependencies, tracker *sa.Tracker, bridge *sa.KernelBridge, registry *sa.CapabilityRegistry, profile config.Profile, appLogger shared_ports.Logger, scannerSvc *aggregator.ScannerService) domain_skills.Registry {
	// Setup Hexagonal Task Engine Middleware Pipeline
	osTranslator := translator.NewOSTranslatorAdapter("") // default to current OS
	
	// Create AI Reviewer for the Thinking phase
	aiReviewer := security.NewAIReviewer(deps.LLM, profile.Provider, appLogger)
	
	taskWarden := security.NewTaskWardenAdapter(deps.Warden, osTranslator, appLogger)

	taskDispatcher := taskengine.NewDispatcher(osTranslator, taskWarden, deps.ShellExecution, aiReviewer, appLogger)

	tools := []struct {
		name string
		err  error
	}{
		{"chat", toolRegistry.RegisterTool(ctx, chat.NewChatTool(deps.LLM, bridge, bridge))},
		{"scan", toolRegistry.RegisterTool(ctx, scan.NewScanTool(scannerSvc))},
		{"subagent", toolRegistry.RegisterTool(ctx, subagent.NewSubagentTool(tracker))},
		{"resume", toolRegistry.RegisterTool(ctx, subagent.NewResumeTool(tracker))},
		{"delegate", toolRegistry.RegisterTool(ctx, delegate.NewDelegateTool(tracker, registry))},
		{"terminal", toolRegistry.RegisterTool(ctx, terminal.NewTerminalTool(taskDispatcher))},
		{"notes", toolRegistry.RegisterTool(ctx, notes.NewNotesTool())},
		{"todo", toolRegistry.RegisterTool(ctx, todo.NewTodoTool())},
		{"file_edit", toolRegistry.RegisterTool(ctx, file_ops.NewFileOpsTool())},
		{"generate_report", toolRegistry.RegisterTool(ctx, reporting.NewReportingTool(deps.LLM))},
	}
	skillRegistry, err := domain_skills.NewEmbeddedRegistry()
	if err != nil {
		appLogger.ErrorErr(ctx, err, "Failed to initialize embedded skill registry")
	} else {
		tools = append(tools, struct {
			name string
			err  error
		}{"load_skill", toolRegistry.RegisterTool(ctx, skills.NewLoadSkillTool(skillRegistry))})
	}

	for _, t := range tools {
		if t.err != nil {
			appLogger.ErrorErr(context.Background(), t.err, "Tool registration failed", shared_ports.Field{Key: "tool", Value: t.name})
			log.Fatalf("%s tool registration failed: %v", t.name, t.err)
		}
	}

	return skillRegistry
}
