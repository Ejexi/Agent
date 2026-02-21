---
trigger: always_on
---

# Adapter Rules

Adapters connect system to external world.

Adapters must:

- implement ports
- contain no business logic

Adapters allowed:

RabbitMQ adapter
gRPC adapter
LLM adapter
DB adapter

Adapters forbidden:

business logic
decision making