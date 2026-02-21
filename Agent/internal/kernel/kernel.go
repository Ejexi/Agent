package kernel

import (
	types "agent/internal/Types"
	"agent/internal/domain"
	"agent/internal/ports"
	"context"
)

// Dependencies holds all external ports needed by the kernel.
type Dependencies struct {
	MessageBus ports.BusPort
	Memory     ports.MemoryPort
	LLM        ports.LLMRegistry
}

// Kernel is the execution authority coordinates registry, runtime and dispatching.
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
	disp := NewDispatcher(run, deps.MessageBus)

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
func (k *Kernel) StartDispatcher(ctx context.Context, topic string) error {
	if k.Deps.MessageBus == nil {
		// return fmt.Errorf("message bus is not configured")
		return types.New(types.ErrCodeInternal, "message bus is not configured")
	}
	return k.dispatcher.Start(ctx, topic)
}

// Execute provides a direct way to execute a tool, mainly used by CLI.
// Notice: Kernel is the only component allowed to execute tools.
func (k *Kernel) Execute(ctx context.Context, task domain.Task) (domain.Result, error) {
	if k.runtime == nil {
		return domain.Result{}, types.New(types.ErrCodeInternal, "runtime is not configured")
	}
	return k.runtime.Execute(ctx, task)
}
