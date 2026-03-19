package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SecDuckOps/agent/internal/adapters/configsync"
	"github.com/SecDuckOps/agent/internal/adapters/events"
	"github.com/SecDuckOps/agent/internal/adapters/executor"
	mcp_adapter "github.com/SecDuckOps/agent/internal/adapters/mcp"
	checkpoint_adapter "github.com/SecDuckOps/agent/internal/adapters/checkpoint"
	"github.com/SecDuckOps/agent/internal/adapters/security"
	sa "github.com/SecDuckOps/agent/internal/adapters/subagent"
	"github.com/SecDuckOps/agent/internal/adapters/translator"
	warden_adapter "github.com/SecDuckOps/agent/internal/adapters/warden"
	agent_app "github.com/SecDuckOps/agent/internal/application"
	"github.com/SecDuckOps/agent/internal/application/taskengine"
	"github.com/SecDuckOps/agent/internal/agent"
	"github.com/SecDuckOps/agent/internal/config"
	mcp_domain "github.com/SecDuckOps/agent/internal/domain/mcp"
	domain_security "github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/tools/implementations/chat"
	"github.com/SecDuckOps/agent/internal/tools/implementations/delegate"
	"github.com/SecDuckOps/agent/internal/tools/implementations/file_ops"
	"github.com/SecDuckOps/agent/internal/tools/implementations/filesystem"
	mcp_tool "github.com/SecDuckOps/agent/internal/tools/implementations/mcp_tool"
	"github.com/SecDuckOps/agent/internal/tools/implementations/search"
	"github.com/SecDuckOps/agent/internal/tools/implementations/notes"
	"github.com/SecDuckOps/agent/internal/tools/implementations/reporting"
	"github.com/SecDuckOps/agent/internal/tools/implementations/skills"
	"github.com/SecDuckOps/agent/internal/tools/implementations/subagent"
	"github.com/SecDuckOps/agent/internal/tools/implementations/terminal"
	"github.com/SecDuckOps/agent/internal/tools/implementations/todo"
	"github.com/SecDuckOps/agent/internal/tools/implementations/web_search"
	domain_skills "github.com/SecDuckOps/agent/internal/skills"
	"github.com/google/uuid"

	"time"

	llm_application "github.com/SecDuckOps/shared/llm/application"
	llm_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/llm/infrastructure"
	"github.com/SecDuckOps/shared/logger"
	shared_ports "github.com/SecDuckOps/shared/ports"
	scanner_ports "github.com/SecDuckOps/shared/scanner/ports"
)

// App holds the initialized application components.
type App struct {
	Kernel          *kernel.Kernel
	Sessions        ports.SessionManager
	AppSessions     ports.AppSessionManager
	MasterAgent     *agent.MasterAgent
	DuckOpsAgent    *agent.DuckOpsAgent
	MCPClient       ports.MCPClientPort
	CheckpointStore *checkpoint_adapter.Store // nil if init failed
	Provider        string
	Model           string
	Logger          shared_ports.Logger
	EventBus        ports.EventBusPort
	SkillRegistry   domain_skills.Registry
	Shutdown        func()
}

// FromTOML bootstraps the application from ~/.duckops/config.toml.
func FromTOML(parentCtx context.Context, tomlCfg *config.DuckOpsConfig) (*App, error) {
	ctx, cancel := context.WithCancel(parentCtx)
	dir, _ := config.DuckOpsDir()
	logPath := filepath.Join(dir, "duckops.log")

	appLogger, err := logger.New("duckops-agent", "info", logPath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	eventBus := events.NewInMemoryEventBus(appLogger)
	appSessionManager := agent_app.NewSessionManagerService(appLogger, eventBus)

	profile, ok := tomlCfg.GetProfile("default")
	if !ok {
		cancel()
		return nil, fmt.Errorf("no 'default' profile found in config.toml")
	}

	cwd, _ := os.Getwd()
	_, err = appSessionManager.CreateSession(ctx, cwd, tomlCfg.Settings.AgentMode, profile.Model)
	if err != nil {
		appLogger.ErrorErr(ctx, err, "Failed to create root agent session")
	}

	capabilityRegistry := sa.NewCapabilityRegistry()

	// ── Super Duck: remote config sync ──────────────────────────────────────
	if tomlCfg.Settings.AgentMode == "super" {
		appLogger.Info(ctx, "Starting in Super Duck mode", shared_ports.Field{Key: "url", Value: tomlCfg.Settings.APIGatewayURL})
		syncAdapter := configsync.NewHTTPAdapter(tomlCfg.Settings.APIGatewayURL, "") // TODO: API Key
		remoteCfg, err := syncAdapter.FetchRemoteConfig(ctx)
		if err != nil {
			appLogger.ErrorErr(ctx, err, "Failed to fetch remote config, falling back to local")
		} else {
			capabilityRegistry.Sync(remoteCfg.Capabilities)
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
					}
				}
			}
		}()
	}

	// ── LLM Registry ─────────────────────────────────────────────────────────
	llmRegistry, err := buildLLMRegistry(profile, appLogger)
	if err != nil {
		cancel()
		return nil, err
	}

	// ── Warden (Cedar policy evaluator — KEEP for shell/fs policy) ───────────
	useDefaultDeny := true
	if profile.Warden != nil {
		useDefaultDeny = profile.Warden.DefaultDeny
	}
	wardenInstance := warden_adapter.New(useDefaultDeny, appLogger)

	var policies []domain_security.NetworkPolicy
	if profile.Warden != nil && len(profile.Warden.PolicyFiles) > 0 {
		for _, path := range profile.Warden.PolicyFiles {
			content, err := os.ReadFile(path)
			if err != nil {
				appLogger.ErrorErr(ctx, err, "Failed to read policy file", shared_ports.Field{Key: "path", Value: path})
				continue
			}
			policies = append(policies, domain_security.NetworkPolicy{
				ID: uuid.New().String(), Name: filepath.Base(path),
				CedarBody: string(content), Enabled: true, Priority: 10,
			})
		}
	}
	if err := wardenInstance.LoadPolicies(ctx, policies); err != nil {
		appLogger.ErrorErr(ctx, err, "Failed to load Warden policies")
	}

	// ── OS executor ──────────────────────────────────────────────────────────
	osExecutor := executor.NewOSExecAdapter(appLogger)
	toolRegistry := agent_app.NewToolRegistryService(appLogger)

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
		cancel()
		return nil, fmt.Errorf("kernel initialization failed")
	}

	bridge := &sa.KernelBridge{
		ExecuteFn:    k.ExecuteCompat,
		GetSchemasFn: k.GetToolSchemas,
		LLMRegistry:  llmRegistry,
	}

	var secretScanner ports.SecretScannerPort
	tracker := sa.NewTracker(bridge, bridge, secretScanner, appLogger)
	// Apply profile-level auto_approve tools to every spawned session
	if len(profile.AutoApproveTools) > 0 {
		tracker.AutoApproveTools = profile.AutoApproveTools
		appLogger.Info(ctx, "Auto-approve tools loaded",
			shared_ports.Field{Key: "tools", Value: profile.AutoApproveTools})
	}

	// Wire checkpoint store — sessions persisted to ~/.duckops/sessions/
	sessionsDir := filepath.Join(dir, "sessions")
	var cpStore *checkpoint_adapter.Store
	if store, err := checkpoint_adapter.NewStore(sessionsDir); err != nil {
		appLogger.ErrorErr(ctx, err, "Failed to initialize checkpoint store — session history will not be persisted")
	} else {
		cpStore = store
		tracker.WithCheckpointStore(cpStore)
		appLogger.Info(ctx, "Checkpoint store ready", shared_ports.Field{Key: "dir", Value: sessionsDir})
	}

	// ── MCP Client ───────────────────────────────────────────────────────────
	// Convert config entries to domain types
	mcpConfigs := make([]mcp_domain.ServerConfig, len(tomlCfg.MCP.Servers))
	for i, s := range tomlCfg.MCP.Servers {
		mcpConfigs[i] = mcp_domain.ServerConfig{
			Name: s.Name, Transport: s.Transport,
			Command: s.Command, URL: s.URL,
			Env: s.Env, AllowedTools: s.AllowedTools,
			Enabled: s.Enabled,
		}
	}
	mcpClient := mcp_adapter.NewClient(mcpConfigs, appLogger)
	appLogger.Info(ctx, "MCP client initialized",
		shared_ports.Field{Key: "connected_servers", Value: mcpClient.ConnectedServers()})

	// ── MasterAgent: uses MCP scanner adapter if scanner server is configured ─
	var scannerSvc scanner_ports.ScannerServicePort
	for _, srv := range tomlCfg.MCP.Servers {
		if srv.Enabled && srv.Name == "scanner" {
			scannerSvc = mcp_adapter.NewScannerAdapter(mcpClient, "scanner")
			appLogger.Info(ctx, "Scanner service: MCP backend active")
			break
		}
	}
	if scannerSvc == nil {
		appLogger.Info(ctx, "Scanner service: not configured — security scans disabled (add [[mcp.servers]] name='scanner')")
	}

	masterAgent := agent.NewMasterAgent(scannerSvc, llmRegistry.Default(), appLogger)
	reportAgent := agent.NewReportAgent(nil, appLogger)
	duckOpsAgent := agent.NewDuckOpsAgent(masterAgent, reportAgent, appLogger)

	// ── Register tools ───────────────────────────────────────────────────────
	skillRegistry, err := registerTools(ctx, toolRegistry, deps, tracker, bridge,
		capabilityRegistry, profile, appLogger, mcpClient)
	if err != nil {
		cancel()
		return nil, err
	}

	provider := profile.Provider
	if provider == "" {
		if providers := llmRegistry.List(); len(providers) > 0 {
			provider = providers[0]
		}
	}

	appLogger.Info(ctx, "🦆 DuckOps Agent initialized successfully")
	return &App{
		Kernel:          k,
		Sessions:        tracker,
		AppSessions:     appSessionManager,
		MasterAgent:     masterAgent,
		DuckOpsAgent:    duckOpsAgent,
		MCPClient:       mcpClient,
		CheckpointStore: cpStore,
		Provider:        provider,
		Model:           profile.Model,
		Logger:          appLogger,
		EventBus:        eventBus,
		SkillRegistry:   skillRegistry,
		Shutdown: func() {
			appLogger.Info(context.Background(), "🛑 Stopping DuckOps Agent...")
			_ = mcpClient.Close()
			cancel()
		},
	}, nil
}

func buildLLMRegistry(profile config.Profile, appLogger shared_ports.Logger) (llm_domain.LLMRegistry, error) {
	sharedCfg := llm_domain.Config{
		Default:   profile.Provider,
		Providers: make(map[string]llm_domain.ProviderConfig),
	}
	for name, prov := range profile.Providers {
		sharedCfg.Providers[name] = llm_domain.ProviderConfig{
			APIKey: prov.APIKey, Model: prov.Model, BaseURL: prov.BaseURL,
		}
	}
	llmRegistry, err := llm_application.NewLLMRegistry(sharedCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM registry: %w", err)
	}
	for name, prov := range profile.Providers {
		if (name == "gemini" || prov.Type == "gemini") && prov.APIKey != "" {
			geminiAdapter, err := infrastructure.NewGeminiAdapter(context.Background(), prov.APIKey, prov.Model)
			if err != nil {
				appLogger.ErrorErr(context.Background(), err, "Gemini init failed for "+name)
			} else {
				llmRegistry.Register(geminiAdapter)
			}
		}
	}
	return llmRegistry, nil
}

func registerTools(
	ctx context.Context,
	toolRegistry ports.ToolRegistry,
	deps kernel.Dependencies,
	tracker *sa.Tracker,
	bridge *sa.KernelBridge,
	registry *sa.CapabilityRegistry,
	profile config.Profile,
	appLogger shared_ports.Logger,
	mcpClient ports.MCPClientPort,
) (domain_skills.Registry, error) {

	osTranslator := translator.NewOSTranslatorAdapter("")
	aiReviewer := security.NewAIReviewer(deps.LLM, profile.Provider, appLogger)
	taskWarden := security.NewTaskWardenAdapter(deps.Warden, osTranslator, appLogger)
	taskDispatcher := taskengine.NewDispatcher(osTranslator, taskWarden, deps.ShellExecution, aiReviewer, appLogger)
	fsGate := filesystem.NewWardenGate(deps.Warden, appLogger)

	tools := []struct {
		name string
		err  error
	}{
		{"chat", toolRegistry.RegisterTool(ctx, chat.NewChatTool(deps.LLM, bridge, bridge))},
		{"subagent", toolRegistry.RegisterTool(ctx, subagent.NewSubagentTool(tracker))},
		{"resume", toolRegistry.RegisterTool(ctx, subagent.NewResumeTool(tracker))},
		{"delegate", toolRegistry.RegisterTool(ctx, delegate.NewDelegateTool(tracker, registry))},
		{"terminal", toolRegistry.RegisterTool(ctx, terminal.NewTerminalTool(taskDispatcher))},
		{"notes", toolRegistry.RegisterTool(ctx, notes.NewNotesTool())},
		{"todo", toolRegistry.RegisterTool(ctx, todo.NewTodoTool())},
		{"file_edit", toolRegistry.RegisterTool(ctx, file_ops.NewFileOpsTool(fsGate))},
		{"generate_report", toolRegistry.RegisterTool(ctx, reporting.NewReportingTool(deps.LLM))},
		// MCP tools — let the LLM call any connected MCP server
		{"mcp_call", toolRegistry.RegisterTool(ctx, mcp_tool.NewMCPTool(mcpClient))},
		{"mcp_list", toolRegistry.RegisterTool(ctx, mcp_tool.NewMCPListTool(mcpClient))},
		{"grep_search", toolRegistry.RegisterTool(ctx, search.NewGrepSearchTool())},
		{"web_search", toolRegistry.RegisterTool(ctx, web_search.NewWebSearchTool())},
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
			return nil, fmt.Errorf("tool registration failed for %s: %w", t.name, t.err)
		}
	}
	return skillRegistry, nil
}
