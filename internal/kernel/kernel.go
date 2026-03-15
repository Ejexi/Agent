package kernel

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
	shared_ports "github.com/SecDuckOps/shared/ports"
	types "github.com/SecDuckOps/shared/types"
)

// Dependencies holds all external ports needed by the kernel.
type Dependencies struct {
	ToolRegistry   ports.ToolRegistry
	MessageBus     ports.BusPort
	Memory         ports.MemoryPort
	LLM            shared_domain.LLMRegistry
	Logger         shared_ports.Logger
	AuditLog       ports.AuditLogPort
	Warden         ports.WardenPort
	ShellExecution ports.ShellExecutionPort
	ShellLifecycle ports.ShellLifecyclePort
}

// Kernel is the execution authority — it coordinates registry, runtime and dispatching.
// The Kernel ONLY executes tools. It does NOT create or manage agents.
type Kernel struct {
	registry   ports.ToolRegistry
	runtime    *Runtime
	dispatcher *Dispatcher

	Deps Dependencies
}

// New creates a new Kernel instance.
func New(deps Dependencies) *Kernel {
	if deps.ToolRegistry == nil {
		return nil // Registry is now a required external dependency
	}
	
	run := NewRuntime(deps.ToolRegistry, deps.AuditLog)
	disp := NewDispatcher(run, deps.MessageBus, deps.Logger)

	if run == nil || disp == nil {
		return nil
	}
	return &Kernel{
		registry:   deps.ToolRegistry,
		runtime:    run,
		dispatcher: disp,
		Deps:       deps,
	}
}

// RegisterTool adds a new tool to the kernel's registry.
func (k *Kernel) RegisterTool(ctx context.Context, tool domain.Tool) error {
	if k.registry == nil {
		return types.New(types.ErrCodeInternal, "registry is not .DuckOpsConfigured")
	}
	return k.registry.RegisterTool(ctx, tool)
}

// StartDispatcher starts the internal dispatcher to listen for tasks.
// inTopic is where incoming commands arrive, and outTopic is where results are published.
func (k *Kernel) StartDispatcher(ctx context.Context, inTopic, outTopic string) error {
	if k.Deps.MessageBus == nil {
		return types.New(types.ErrCodeInternal, "message bus is not .DuckOpsConfigured")
	}
	return k.dispatcher.Start(ctx, inTopic, outTopic)
}

// SetMessageBus allows late-binding of the message bus adapter to the kernel.
func (k *Kernel) SetMessageBus(bus ports.BusPort) {
	k.Deps.MessageBus = bus
	if k.dispatcher != nil {
		k.dispatcher.bus = bus
	}
}

// Execute provides a direct way to execute a tool, mainly used by CLI.
// Notice: Kernel is the only component allowed to execute tools.
func (k *Kernel) Execute(ctx *ExecutionContext, task domain.Task) (domain.Result, error) {
	if k.runtime == nil {
		return domain.Result{}, types.New(types.ErrCodeInternal, "runtime is not .DuckOpsConfigured")
	}
	return k.runtime.Execute(ctx, task)
}

// ExecuteBatch provides a way to execute multiple tools in parallel.
func (k *Kernel) ExecuteBatch(ctx *ExecutionContext, tasks []domain.Task) ([]domain.Result, error) {
	if k.runtime == nil {
		return nil, types.New(types.ErrCodeInternal, "runtime is not .DuckOpsConfigured")
	}
	return k.runtime.ExecuteBatch(ctx, tasks)
}

// ExecuteCompat satisfies the ports.ToolExecutor interface using context.Context.
// It wraps the incoming context into an ExecutionContext with system-level defaults.
// This is used by subagents and external consumers that don't manage capabilities directly.
func (k *Kernel) ExecuteCompat(ctx context.Context, task domain.Task) (domain.Result, error) {
	execCtx := NewExecutionContext(ctx, task.SessionID, "system:compat", nil) // nil caps = no restrictions
	return k.Execute(execCtx, task)
}

// GetToolSchemas returns the schemas of all registered tools.
// If allowedTools is non-empty, only schemas for those tools are returned.
func (k *Kernel) GetToolSchemas(allowedTools []string) []domain.ToolSchema {
	if k.registry == nil {
		return nil
	}

	schemasResult, err := k.registry.ListTools(context.Background())
	if err != nil {
		return nil
	}

	if len(allowedTools) == 0 {
		return schemasResult
	}

	schemas := make([]domain.ToolSchema, 0, len(schemasResult))
	allowed := make(map[string]bool, len(allowedTools))
	for _, name := range allowedTools {
		allowed[name] = true
	}

	for _, schema := range schemasResult {
		if allowed[schema.Name] {
			schemas = append(schemas, schema)
		}
	}
	return schemas
}
