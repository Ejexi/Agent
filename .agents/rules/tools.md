---
trigger: always_on
---

# Tool System Contract

Tools are execution units of the agent.

Tools must:

- be stateless
- receive Task
- return Result

Tools must not:

- contain infrastructure logic
- contain orchestration logic

Kernel handles orchestration.

Example Tool:

ScanTool

RemediationTool

QueryTool