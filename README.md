# DuckOps Agent: The Autonomous Execution Engine

The DuckOps Agent is a high-performance, stateless worker engine designed to execute security tasks in isolated environments. It is built using a strict **Kernel-Policy Architecture**.

---

## 🧠 1. Internal Engine Components

The Agent is NOT just a CLI; it's an engine with three core internal components:

### **The Registry** (`/kernel/registry.go`)

- The "Library of Capabilities".
- It stores maps of `domain.Tool` implementations.
- Every tool must register its name and instance here during the `bootstrap` phase.

### **The Runtime** (`/kernel/runtime.go`)

- The "Safety Wrapper".
- Responsible for the actual execution of `tool.Run()`.
- Built-in **Panic Recovery**: It catches tool panics and converts them into structured `AppError` objects.
- **Metrics & Logging**: It transparently logs tool execution time and status without modifying the tool code.

### **The Dispatcher** (`/kernel/dispatcher.go`)

- The "Task Router".
- Used in `cloud` mode to poll the **MessageBus Port** (RabbitMQ).
- It unmarshals incoming tasks and hands them to the Kernel for execution.

---

## 🛠️ 2. Detailed Tool Development Guide

Every tool must implement the `domain.Tool` interface.

### **Golden Rules for Tools**

1.  **Must be Stateless**: Tools should never store data across executions.
2.  **No direct I/O**: Use injected `Ports` (Filesystem, LLM, Memory) for all external interactions.
3.  **AppError Enforcement**: Never return raw errors. Use `types.New` or `types.Wrap`.

### **Implementation Example**

```go
type SecurityScanner struct {
    llm ports.LLM // Injected Port
}

func (s *SecurityScanner) Run(ctx context.Context, task domain.Task) (domain.Result, error) {
    // 1. Parameter extraction
    target, ok := task.Args["target"].(string)
    if !ok {
        return domain.Result{}, types.New(types.ErrCodeInvalidInput, "missing target path")
    }

    // 2. Business Logic using Ports
    finding, err := s.llm.Generate(ctx, "Scan this: " + target)
    if err != nil {
        return domain.Result{}, types.Wrap(err, types.ErrCodeToolFailed, "llm analysis failed")
    }

    // 3. Structured Result return
    return domain.Result{Success: true, Data: finding}, nil
}
```

---

## 🔄 3. Granular Execution Lifecycle

When `kernel.Execute(task)` is called:

1.  **Lookup**: Kernel asks the **Registry** for the tool matching `task.ToolName`.
2.  **Wrappers**: Runtime wraps the execution with a `defer recover()` block.
3.  **Injection**: Context (with Correlation ID) is passed down to the tool.
4.  **Execution**: `tool.Run()` is called.
5.  **Post-Processing**: Runtime captures the `Result`, logs the performance metadata, and flushes traces.
6.  **Return**: The clean `domain.Result` is returned to the original caller (CLI or Bus Adapter).

---

## 🔗 Connections

### **Dependency: Shared Foundation**

This repository depends on **[DuckOps Shared](https://github.com/SecDuckOps/shared)**.
It provides the shared `AppError` system, LLM ports, and event types used to communicate with the server.

For local development:

```bash
# Example go.mod replace
replace github.com/SecDuckOps/shared => ../shared
```

### **Interaction: DuckOps Server**

The Agent is a worker that interacts with the **[Server](https://github.com/SecDuckOps/server)** asynchronously via RabbitMQ.
It is essentially a "Remote Procedure Call" target for the Server's Orchestrator. It remains completely unaware of the Server's persistence or state machine.

---

_DuckOps Agent: Secure, Isolated, Autonomous._
