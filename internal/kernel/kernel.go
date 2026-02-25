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
	MessageBus ports.BusPort
	Memory     ports.MemoryPort
	LLM        shared_domain.LLMRegistry
	Logger     shared_ports.Logger
}

// Kernel is the execution authority — it coordinates registry, runtime and dispatching.
// The Kernel ONLY executes tools. It does NOT create or manage agents.
type Kernel struct {
	registry   *Registry
	runtime    *Runtime
	dispatcher *Dispatcher

	Deps Dependencies
}

// New creates a new Kernel instance.
func New(deps Dependencies) *Kernel {
	reg := NewRegistry()
	run := NewRuntime(reg)
	disp := NewDispatcher(run, deps.MessageBus, deps.Logger)

	if reg == nil || run == nil || disp == nil {
		return nil
	}
	return &Kernel{
		registry:   reg,
		runtime:    run,
		dispatcher: disp,
		Deps:       deps,
	}
}

// RegisterTool adds a new tool to the kernel's registry.
func (k *Kernel) RegisterTool(tool domain.Tool) error {
	if k.registry == nil {
		return types.New(types.ErrCodeInternal, "registry is not configured")
	}
	return k.registry.Register(tool)
}

// StartDispatcher starts the internal dispatcher to listen for tasks.
// inTopic is where incoming commands arrive, and outTopic is where results are published.
func (k *Kernel) StartDispatcher(ctx context.Context, inTopic, outTopic string) error {
	if k.Deps.MessageBus == nil {
		return types.New(types.ErrCodeInternal, "message bus is not configured")
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
func (k *Kernel) Execute(ctx context.Context, task domain.Task) (domain.Result, error) {
	if k.runtime == nil {
		return domain.Result{}, types.New(types.ErrCodeInternal, "runtime is not configured")
	}
	return k.runtime.Execute(ctx, task)
}

// ExecuteBatch provides a way to execute multiple tools in parallel.
func (k *Kernel) ExecuteBatch(ctx context.Context, tasks []domain.Task) ([]domain.Result, error) {
	if k.runtime == nil {
		return nil, types.New(types.ErrCodeInternal, "runtime is not configured")
	}
	return k.runtime.ExecuteBatch(ctx, tasks)
}

// GetToolSchemas returns the schemas of all registered tools.
// If allowedTools is non-empty, only schemas for those tools are returned.
func (k *Kernel) GetToolSchemas(allowedTools []string) []domain.ToolSchema {
	if k.registry == nil {
		return nil
	}

	allTools := k.registry.ListAll()
	schemas := make([]domain.ToolSchema, 0, len(allTools))

	allowed := make(map[string]bool, len(allowedTools))
	for _, name := range allowedTools {
		allowed[name] = true
	}

	for _, tool := range allTools {
		if len(allowedTools) > 0 && !allowed[tool.Name()] {
			continue
		}
		schemas = append(schemas, tool.Schema())
	}
	return schemas
}
