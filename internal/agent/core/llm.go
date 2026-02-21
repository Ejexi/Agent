package core

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v2"
	"go.uber.org/zap"
)

//======================LLM COMMUNICATION METHODS=========================

// buildMessages constructs the full message array for LLM
// Combines: system prompt + conversation history
func (a *Agent) buildMessages() []openai.ChatCompletionMessageParamUnion {
	history := a.memory.GetHistory()

	// Allocate space: system prompt + history
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(history)+1)

	// First: System prompt (tells LLM who it is + available tools)
	messages = append(messages, openai.SystemMessage(a.buildSystemPrompt()))

	// Then: Conversation history
	for _, msg := range history {
		switch msg.Role {
		case "user":
			messages = append(messages, openai.UserMessage(msg.Content))
		case "assistant":
			messages = append(messages, openai.AssistantMessage(msg.Content))
		}
	}

	return messages
}

// buildSystemPrompt creates instructions for the LLM
// Tells LLM: identity + available tools + how to use them
func (a *Agent) buildSystemPrompt() string {
	// Get all registered tools
	tools := a.registry.List()

	// Build tool list with schemas
	toolList := ""
	for _, tool := range tools {
		schema := tool.Schema()
		toolList += fmt.Sprintf("\n  - %s: %s", tool.Name(), tool.Description())

		// Add parameters info
		if len(schema.Parameters.Properties) > 0 {
			toolList += "\n    Parameters:"
			for paramName, paramSchema := range schema.Parameters.Properties {
				required := ""
				for _, req := range schema.Parameters.Required {
					if req == paramName {
						required = " (required)"
						break
					}
				}
				toolList += fmt.Sprintf("\n      • %s (%s)%s: %s",
					paramName,
					paramSchema.Type,
					required,
					paramSchema.Description,
				)
			}
		}
		toolList += "\n"
	}

	if toolList == "" {
		toolList = "\n  No tools available\n"
	}

	return fmt.Sprintf(`You are an AI DevSecOps assistant.
You help with security scanning, CI/CD monitoring, vulnerability detection, and DevSecOps automation.

Available Tools:%s
How to Use Tools:
  When you need to use a tool, respond EXACTLY in this format:
  
  TOOL:<tool_name>|<json_params>
  
  Examples:
  - TOOL:echo|{"message":"Hello World"}
  - TOOL:security_scan|{"target":"/path/to/code","depth":"full"}

Rules:
  1. Use a tool ONLY when the user explicitly asks for an action
  2. Tool command must be on a single line
  3. Parameters must be valid JSON
  4. If no tool is needed, respond conversationally
  5. After tool execution, explain the results clearly
  6. Keep responses concise and professional

Important:
  - The JSON must be valid (use double quotes, proper escaping)
  - Required parameters must be included
  - Tool name must match exactly`, toolList)
}

// callLLM sends the message array to LLM and returns the response
func (a *Agent) callLLM(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (string, error) {
	a.logger.Debug("Calling LLM",
		zap.String("model", a.model),
		zap.Int("message_count", len(messages)),
	)

	// Build request parameters
	params := openai.ChatCompletionNewParams{
		Model:    a.model,
		Messages: messages,
	}

	// Call LLM API
	response, err := a.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("LLM API call failed: %w", err)
	}

	// Validate response
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("LLM returned empty response")
	}

	content := response.Choices[0].Message.Content

	a.logger.Debug("LLM response received",
		zap.Int("length", len(content)),
	)

	return content, nil
}
