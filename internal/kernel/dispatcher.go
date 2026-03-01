package kernel

import (
	"context"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	shared_ports "github.com/SecDuckOps/shared/ports"
	types "github.com/SecDuckOps/shared/types"
)

// Dispatcher processes incoming tasks from the message bus.
type Dispatcher struct {
	runtime *Runtime
	bus     ports.BusPort
	logger  shared_ports.Logger
}

// NewDispatcher creates a new dispatcher.
func NewDispatcher(runtime *Runtime, bus ports.BusPort, logger shared_ports.Logger) *Dispatcher {
	return &Dispatcher{
		runtime: runtime,
		bus:     bus,
		logger:  logger,
	}
}

// Start begins listening for tasks on the inTopic and publishes results to outTopic.
func (d *Dispatcher) Start(ctx context.Context, inTopic, outTopic string) error {
	if d.bus == nil {
		return types.New(types.ErrCodeInternal, "message bus is not .DuckOpsConfigured for dispatcher")
	}

	// The bus adapter handles deserialization — we receive a clean domain.Task
	err := d.bus.Subscribe(ctx, inTopic, func(task domain.Task) {
		result, err := d.runtime.Execute(ctx, task)
		if err != nil && d.logger != nil {
			d.logger.ErrorErr(ctx, "operation_failed", err, "Task execution failed")
		}

		// Ensure the Task ID is part of the result for correlation on the server
		if result.Data == nil {
			result.Data = make(map[string]interface{})
		}
		result.Data["scan_id"] = task.ID

		if d.bus != nil {
			pubErr := d.bus.Publish(ctx, outTopic, result)
			if pubErr != nil && d.logger != nil {
				d.logger.ErrorErr(ctx, "operation_failed", pubErr, "Failed to publish result")
			}
		}
	})

	if err != nil {
		return types.Wrapf(err, types.ErrCodeInternal, "failed to subscribe to topic: %s", inTopic)
	}

	return nil
}
