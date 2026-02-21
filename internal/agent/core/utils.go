package core

import (
	"github.com/Ejexi/Agent/internal/agent/memory"
	"github.com/Ejexi/Agent/internal/tools/base"
)

// ═══════════════════════════════════════════════════════════
//              UTILITY METHODS
// ═══════════════════════════════════════════════════════════

// ClearMemory clears the conversation history
func (a *Agent) ClearMemory() {
	a.memory.Clear()
	a.logger.Info("Conversation memory cleared")
}

// GetMemoryCount returns the number of messages in memory
func (a *Agent) GetMemoryCount() int {
	return a.memory.Count()
}

// GetMemoryHistory returns the full conversation history
func (a *Agent) GetMemoryHistory() []memory.Message {
	return a.memory.GetHistory()
}

// ListTools returns the names of all registered tools
func (a *Agent) ListTools() []string {
	tools := a.registry.List()
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name())
	}
	return names
}

// GetToolSchema returns the schema for a specific tool
func (a *Agent) GetToolSchema(toolName string) (base.ToolSchema, error) {
	tool, err := a.registry.Get(toolName)
	if err != nil {
		return base.ToolSchema{}, err
	}
	return tool.Schema(), nil
}

// ListToolSchemas returns schemas for all registered tools
func (a *Agent) ListToolSchemas() []base.ToolSchema {
	tools := a.registry.List()
	schemas := make([]base.ToolSchema, 0, len(tools))
	for _, tool := range tools {
		schemas = append(schemas, tool.Schema())
	}
	return schemas
}
