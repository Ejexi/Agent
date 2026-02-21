---
description: Create and register a new tool following Agent Kernel and Hexagonal Architecture
---

# Workflow: Create Tool

Goal:
Create a new tool that integrates with the Agent Kernel.

## Step 1: Create Tool Folder

Path:
internal/tools/{tool_name}

Example:
internal/tools/sast

---

## Step 2: Create Tool File

Path:
internal/tools/{tool_name}/{tool_name}_tool.go

---

## Step 3: Implement Tool Interface

Required interface:

type Tool interface {
    Name() string
    Run(ctx context.Context, task Task) (Result, error)
}

---

## Step 4: Implement Tool Structure

Example:

type {ToolName}Tool struct {}

func New{ToolName}Tool() *{ToolName}Tool {
    return &{ToolName}Tool{}
}

func (t *{ToolName}Tool) Name() string {
    return "{tool_name}"
}

func (t *{ToolName}Tool) Run(ctx context.Context, task Task) (Result, error) {
    // implementation
}

---

## Step 5: Register Tool in Kernel

File:
internal/kernel/kernel.go

Add:

kernel.RegisterTool(New{ToolName}Tool())

---

## Step 6: Verify Integration

Ensure:

- Tool is registered
- Tool compiles
- Tool accessible from CLI

---

## Architecture Constraints

Tool MUST:

- be stateless
- use ports only
- not access adapters directly

Forbidden:

- calling infrastructure directly
- bypassing kernel
- executing tool outside kernel

---

## Execution Flow

CLI → Kernel → Runtime → Tool → Port → Adapter