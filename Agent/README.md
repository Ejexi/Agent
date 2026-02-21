# DevSecOps Agent - Developer Guide

Welcome to the **DevSecOps Agent Engine** developer guide.
This core engine is carefully designed using **Hexagonal Architecture** (Ports and Adapters) combined with an **Event-Driven Architecture**.

The primary goals of this architecture are:

1. **Kernel Isolation:** The Core Kernel is completely agnostic to the outside world (like Databases, message brokers, or external APIs).
2. **Tool Isolation:** Tools do not have the authority to execute themselves, nor do they communicate directly with infrastructure. Instead, they use injected `Ports`.
3. **Ultimate Flexibility:** The Core Engine can be embedded into any service (e.g., CLI, RabbitMQ Worker, RESTful API Server) without changing a single line of internal code.

---

## Architecture Layers

The project is divided into the following strict layers based on our internal rules:

- `internal/domain/`: Contains the core entities of the system (`Task`, `Result`, `Tool`). This is the universal language of the Engine; it has zero external dependencies. **(domain → no dependencies)**
- `internal/ports/`: The Interfaces requested by the Engine to communicate with the outside world (e.g., `MessageBus`, `Memory`, `LLM`). **(ports → domain only)**
- `internal/kernel/`: The mastermind and the **sole authority for execution**. It houses the `Registry` (for storing tools), `Runtime` (the execution environment), and `Dispatcher` (task router). **(kernel → domain only)**
- `internal/Types/`: Contains our robust, structured error system (`AppError`) with traceable error codes.
- `internal/tools/`: The actual DevSecOps tools (e.g., `scan_tool`). These tools are instantiated by injecting specific `Ports` as dependencies. **(tools → ports + domain)**
- `internal/adapters/`: The concrete implementations that connect the outside world to the system (e.g., actual RabbitMQ code that implements the `ports.BusPort` interface). **(adapters → ports only)**
- `internal/config/`: Configuration management module utilizing Viper to read `config.yaml` and OS-level Environment Variables via 12-factor methodology.
- `cmd/`: The application entrypoints. This is where we wire up all the pieces (Adapters + Tools + Kernel) to start a specific service. **(cmd → kernel only)**

** Forbidden Operations (Golden Rules):**

- `domain` or `kernel` must **never** depend on `adapters` or `config`.
- `tools` must **never** access infrastructure, configurations, or adapters directly.
- `tool.Run()` should NEVER be called directly. Always route execution through the Kernel: `kernel.Execute(task)`.

---

## How to Add a New Tool

Every tool must be **stateless**, deterministic, and implement the `domain.Tool` interface:

```go
type Tool interface {
	Name() string
	Run(ctx context.Context, task Task) (Result, error)
}
```

### Step-by-Step Practical Guide:

1. **Create the Tool Directory:** `internal/tools/{tool_name}/`
2. **Define the Struct and Request Ports:**
   Determine which Ports your tool needs (e.g., Memory and LLM). Never use infrastructure packages (like `database/sql` or `net/http`) directly inside the tool.

   ```go
   package sast

   import (
   	"agent/internal/Types"
   	"agent/internal/domain"
   	"agent/internal/ports"
   	"context"
   )

   type SastTool struct {
   	llm ports.LLMRegistry // The injected Port
   }

   func NewSastTool(llm ports.LLMRegistry) *SastTool {
   	return &SastTool{llm: llm} // Dependency Injection
   }

   func (t *SastTool) Name() string { return "sast" }
   ```

3. **Implement the Business Logic (`Run()`) and return a `domain.Result`:**
   - Extract inputs from `task.Args`.
   - Handle errors using `types.New()` or `types.Wrap()`.
   - Use the injected Ports when needed (e.g., `t.llm.Get("openai").Generate()`).

   ```go
   func (t *SastTool) Run(ctx context.Context, task domain.Task) (domain.Result, error) {
   	target, ok := task.Args["target"].(string)
   	if !ok {
   		// Utilizing our Structured Errors
   		return domain.Result{}, types.New(types.ErrCodeInvalidInput, "missing target")
   	}

   	// Business Logic (No infrastructure logic allowed)
   	return domain.Result{
   		TaskID:  task.ID,
   		Success: true,
   		Status:  "done",
   		Data:    map[string]interface{}{"found": true},
   	}, nil
   }
   ```

4. **Register the Tool in the Entrypoint:**
   In your service entrypoint (e.g., `cmd/agent-cli/main.go`), instantiate the tool with its required Ports, then register it to the Kernel:
   ```go
   sastTool := sast.NewSastTool(deps.LLM)
   k.RegisterTool(sastTool)
   ```

---

## How to Add a New Adapter

Adapters connect the system to the external world. **Adapters must contain NO business logic or decision making.**
An Adapter always implements an Interface defined in the `internal/ports/` directory.

### Example: Creating a RabbitMQ Adapter

Suppose you want the Engine to listen to RabbitMQ.
The Architecture prohibits the Kernel from knowing what RabbitMQ is. The Kernel only understands `ports.BusPort`.

1. **Create the Adapter Directory:** `internal/adapters/rabbitmq/`
2. **Implement the Port:**
   Create code that imports the `amqp` package, and build a Struct that matches the `BusPort` interface methods.

   ```go
   package rabbitmq

   import (
   	"agent/internal/domain"
   	"context"
   	"fmt"
   	// import "github.com/streadway/amqp" (for example)
   )

   type RabbitMQAdapter struct {
   	// conn *amqp.Connection
   }

   func NewRabbitMQAdapter(url string) *RabbitMQAdapter {
   	// Initialize the real connection here
   	return &RabbitMQAdapter{}
   }

   // Fulfilling the Publish Interface
   func (r *RabbitMQAdapter) Publish(ctx context.Context, topic string, message interface{}) error {
   	fmt.Println("Publishing via RabbitMQ to", topic)
   	return nil
   }

   // Fulfilling the Subscribe Interface and transforming messages to domain.Task
   func (r *RabbitMQAdapter) Subscribe(ctx context.Context, topic string, handler func(domain.Task)) error {
   	// 1. Listen to the Queue via amqp
   	// 2. Unmarshal the JSON into a domain.Task object
   	// 3. Execute the provided handler
   	// handler(task)
   	return nil
   }
   ```

3. **Inject the Adapter into the Kernel (Dependency Injection):**
   Now, in your service entrypoint (e.g., your Rabbit Worker service):

   ```go
   rabbitBus := rabbitmq.NewRabbitMQAdapter("amqp://localhost")

   deps := kernel.Dependencies{
   	MessageBus: rabbitBus, // The Kernel accepts this via Polymorphism
   }

   k := kernel.New(deps)
   ```

This pattern applies to everything:

- `OpenRouterAdapter` implements `ports.LLM`.
- `PostgresAdapter` implements `ports.MemoryPort`.

---

## ⚙️ Configuration Management (12-Factor App)

The application configures itself using a hybrid YAML + Environment Variable approach managed by **Viper**.

- Base configurations (like models) reside in `config.yaml`.
- Secrets (like API keys) MUST be injected via Environment Variables.

**Format:** `AGENT_` + `YAML_PATH_TO_KEY` (replacing `.` with `_`).

#### Example Usage:

```bash
# Windows
$env:AGENT_LLM_OPENAI_API_KEY="sk-proj..."

# Linux / Mac
export AGENT_LLM_GEMINI_API_KEY="AIzaSy..."

# Run the agent
go run ./cmd/agent-cli/main.go
```

---

## Error Handling Guide

Do not return generic errors (like `fmt.Errorf()`) to upper layers in this project.
Always use the robust, structured error system located in `internal/Types/error.go`. It protects the underlying cause and only serializes to the client what you intend:

- **Creating a new error:**
  ```go
  err := types.New(types.ErrCodeInvalidInput, "this input is invalid")
  ```
- **Creating a formatted error:**
  ```go
  err := types.Newf(types.ErrCodeToolNotFound, "Tool %s not found in registry", toolName)
  ```
- **Wrapping an error from an external library and adding Context for tracing:**
  ```go
  dbErr := db.Save()
  if dbErr != nil {
      return types.Wrap(dbErr, types.ErrCodeInternal, "failed to save to database").WithContext("query", "INSERT...")
  }
  ```

---

## Execution Flow Summary

A reminder of how the flow works (whether it's CLI, REST, or RabbitMQ):

**Execution Flow (Golden Rule Enforcement):**
`CLI → Kernel → Runtime → Tool → Message Bus → Worker → Result`

1. The outside world sends a request.
2. The **Adapter / Entrypoint** transforms the request into a `domain.Task`.
3. The **Entrypoint** submits the `Task` to `kernel.Execute()` (or `kernel.StartDispatcher()`).
4. The **Kernel** pushes the `Task` into the **Runtime**.
5. The **Runtime** fetches the `Tool` from the **Registry** using the tool's name (`Task.Tool`).
6. The **Runtime** encapsulates the execution and calls `tool.Run(ctx, task)`.
7. The **Tool** performs its job, utilizing any injected **Ports** for communication or storage.
8. The **Tool** returns a clean `domain.Result` that has zero infrastructure dependencies.
9. The **Runtime** catches the result (and intercepts any `Types.AppError` gracefully), returning it to the **Kernel**.
10. The **Kernel** returns it to the **Adapter**, which then handles dispatching it outward (e.g., as an HTTP JSON Response or an AMQP message).

This architecture guarantees resilience and flexibility. Every single piece is decoupled, easily testable via mocks, and fully isolated. Happy coding!
