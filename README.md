# DevSecOps AI Agent

## 📖 Overview

This project is a DevSecOps AI Agent designed with a **Hexagonal Architecture** to ensure scalability, maintainability, and clean separation of concerns. It is built in Go and is structured to allow the team to easily extend its capabilities, particularly by adding new tools and interacting with core components.

---

## 🏗️ Architecture

The project follows a modular architecture where the core logic is isolated from external dependencies (tools, interfaces).

### Core Components

The core logic resides in `internal/core` and `internal/agent`. These modules provide the foundational services like configuration, logging, and memory management.

### Hexagonal Layers

- **Domain/Core**: `internal/agent/memory`, `internal/core` (Business logic and core utilities)
- **Ports (Interfaces)**: `internal/tools/base` (Defines how tools interact with the core)
- **Adapters (Implementations)**: `internal/tools/implementations` (Concrete tool logic), `cmd/agent` (CLI Adapter)

---

## 📂 Project Structure & Usable Functions

This section lists the key files and the functions available for the team to use. **Use these functions to interact with the system; do not bypass them.**

### 1. Configuration (`internal/core/config/config.go`)

Handles application configuration using Viper.

- **`Load(configPath string) (*Config, error)`**
  - Loads configuration from a YAML file and environment variables.
  - _Usage_: Call this at startup to initialize the app config.

### 2. Logger (`internal/core/logger/logger.go`)

Structured logging wrapper around Zap.

- **`New(level string) (*Logger, error)`**
  - Creates a new logger instance. `level` can be "debug", "info", "warn", "error".
- **`Debug(msg string, fields ...zap.Field)`**
- **`Info(msg string, fields ...zap.Field)`**
- **`Error(msg string, fields ...zap.Field)`**
  - _Usage_: Use these for all logging. Avoid `fmt.Println`.

### 3. Agent Memory (`internal/agent/memory/memory.go`)

Manages the conversation history (Context).

- **`New(maxMessages int) *Memory`**
  - Creates a new thread-safe memory instance.
- **`AddMessage(msg Message)`**
  - Adds a user/system/assistant message to the history, respecting the limit.
- **`GetHistory() []Message`**
  - Returns a copy of the current message history.
- **`Clear()`**
  - Wipes the memory.
- **`Count() int`**
  - Returns the number of stored messages.

### 4. Tool Registry (`internal/tools/registry/registry.go`)

The central hub for managing available tools.

- **`New() *Registry`**
  - Initializes a new empty registry.
- **`Register(tool base.Tool) error`**
  - Adds a new tool to the system. Returns error if duplicate.
- **`Get(name string) (base.Tool, error)`**
  - Retrieves a tool by its name.
- **`List() []base.Tool`**
  - Returns all registered tools.
- **`Has(name string) bool`**
  - Checks if a tool is registered.

### 5. Base Tool Interface (`internal/tools/base/tool.go`)

The contract that all tools must satisfy.

- **`NewBaseTool(name, description string) *BaseTool`**
  - Helper to create the common base structure for any new tool.

---

## 🧩 How to Add a New Tool

To extend the agent with new capabilities:

1.  Create a new directory in `internal/tools/implementations/<your_tool_name>`.
2.  Create a file `<your_tool_name>.go`.
3.  Define a struct that embeds `*base.BaseTool`.
4.  Implement the `Execute` and `Validate` methods as defined in the `Tool` interface.
5.  Register it in `cmd/agent/main.go` (or wherever the startup logic resides) using `registry.Register()`.

**Example:**
See `internal/tools/implementations/echo/echo.go` for a reference implementation.

---

## 📚 Libraries & Dependencies

The project relies on these key libraries. Add new ones here if you introduce major dependencies.

- **[`viper`](https://github.com/spf13/viper)**: For configuration management (handling YAML, Env vars).
- **[`zap`](https://go.uber.org/zap)**: For fast, structured logging.
- **[`cobra`](https://github.com/spf13/cobra)**: (Planned) For CLI command handling.

---

## 🚀 Getting Started

1.  **Install Go**: Ensure Go 1.25+ is installed.
2.  **Install Dependencies**:
    ```bash
    go mod download
    ```
3.  **Run the Agent**:
    ```bash
    go run cmd/agent/main.go
    ```
