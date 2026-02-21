---
trigger: always_on
---

# System Rules

Kernel Rules:

- Kernel is the execution authority
- All tools must be executed via kernel

Tool Rules:

All tools must implement:

type Tool interface {

    Name() string

    Run(ctx context.Context, task Task) (Result, error)

}

Tools must:

- be stateless
- be deterministic
- use ports only

Forbidden:

- direct database access
- direct RabbitMQ usage
- direct gRPC usage
- direct filesystem usage without tool abstraction

Adapter Rules:

Adapters must:

- implement ports
- contain no business logic