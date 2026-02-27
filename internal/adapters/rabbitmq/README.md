# adapters/rabbitmq/

RabbitMQ adapter. Implements `ports.BusPort`.

## Purpose

Message bus adapter for asynchronous task dispatching. Used by the Kernel's `Dispatcher` to:

1. **Subscribe** to incoming task commands on a topic
2. **Publish** execution results back to a results topic

## Architecture Role

```
Dispatcher → BusPort (RabbitMQ) → Subscribe(topic, handler)
                                → Publish(topic, result)
```

Enables event-driven, async execution across distributed agent instances.
