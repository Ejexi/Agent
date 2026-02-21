---
trigger: always_on
---

# DevSecOps Agent Kernel System

You are working on a production-grade DevSecOps Agent built in Go.

Architecture:

- Agent Kernel Architecture
- Hexagonal Architecture
- Event Driven Architecture

Core Components:

- Kernel (execution authority)
- Tool Registry
- Runtime Dispatcher
- Tools
- Ports
- Adapters

Golden Rule:

The Kernel is the ONLY component allowed to execute tools.

Correct:

kernel.Execute(task)

Forbidden:

tool.Run()

Execution Flow:

CLI → Kernel → Runtime → Tool → Message Bus → Worker → Result

Never bypass the kernel.

Goals:

- Build modular tools
- Maintain strict architecture boundaries
- Ensure all tools are stateless
- Use ports for external communication
