package kernel

import (
	"context"
	"log"

	"github.com/SecDuckOps/Agent/internal/domain"
	"github.com/SecDuckOps/Agent/internal/ports"
	types "github.com/SecDuckOps/Shared/types"
	shared "github.com/SecDuckOps/Shared"
)

// Dispatcher processes incoming tasks from the message bus.
type Dispatcher struct {
	runtime *Runtime
	bus     ports.BusPort
}

// NewDispatcher creates a new dispatcher.
func NewDispatcher(runtime *Runtime, bus ports.BusPort) *Dispatcher {
	return &Dispatcher{
		runtime: runtime,
		bus:     bus,
	}
}

// Start begins listening for tasks on the message bus.
func (d *Dispatcher) Start(ctx context.Context, topic string) error {
	if d.bus == nil {
		return types.New(types.ErrCodeInternal, "message bus is not configured for dispatcher")
	}

	// The bus adapter handles deserialization — we receive a clean domain.Task
	err := d.bus.Subscribe(ctx, topic, func(task domain.Task) {
		result, err := d.runtime.Execute(ctx, task)
		if err != nil {
			log.Printf("Task execution failed: %v", err)
		}

		// Ensure the Task ID is part of the result for correlation on the server
		if result.Data == nil {
			result.Data = make(map[string]interface{})
		}
		result.Data["scan_id"] = task.ID

		if d.bus != nil {
			pubErr := d.bus.Publish(ctx, shared.QueueTaskResults, result)
			if pubErr != nil {
				log.Printf("Failed to publish result: %v",
					types.Wrap(pubErr, types.ErrCodeInternal, "failed to publish task results"))
			}
		}
	})

	if err != nil {
		return types.Wrapf(err, types.ErrCodeInternal, "failed to subscribe to topic: %s", topic)
	}

	return nil
}
