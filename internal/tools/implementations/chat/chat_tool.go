package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/domain/subagent"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/types"
	"strings"
)

// ChatParams defines the parameters for the chat tool.
type ChatParams struct {
	SystemPrompt    string `json:"system_prompt,omitempty"`
	AssistantPrefix string `json:"assistant_prefix,omitempty"`
	Prompt          string `json:"prompt"`
	AIProvider      string `json:"ai_provider"`
}

// ChatTool implements the base.Tool interface for chatting with an LLM.
type ChatTool struct {
	base.BaseTypedTool[ChatParams]
	llmRegistry    domain.LLMRegistry
	executor       ports.ToolExecutor
	schemaProvider ports.ToolSchemaProvider
}

// NewChatTool creates a new instance of ChatTool.
func NewChatTool(llmRegistry domain.LLMRegistry, executor ports.ToolExecutor, schemaProvider ports.ToolSchemaProvider) *ChatTool {
	t := &ChatTool{
		llmRegistry:    llmRegistry,
		executor:       executor,
		schemaProvider: schemaProvider,
	}
	t.Impl = t
	return t
}

// Name returns the name of the tool.
func (t *ChatTool) Name() string {
	return "chat"
}

// Schema returns the tool schema for LLM function calling.
func (t *ChatTool) Schema() agent_domain.ToolSchema {
	return agent_domain.ToolSchema{
		Name:        "chat",
		Description: "A tool for chatting with various LLM providers.",
		Parameters: map[string]string{
			"system_prompt":    "string",
			"assistant_prefix": "string",
			"prompt":           "string",
			"ai_provider":      "string",
		},
	}
}

// ParseParams parses the raw input into ChatParams.
func (t *ChatTool) ParseParams(input map[string]interface{}) (ChatParams, error) {
	params, err := base.DefaultParseParams[ChatParams](input)
	if err != nil {
		return params, err
	}
	if params.Prompt == "" {
		return params, types.New(types.ErrCodeInvalidInput, "missing 'prompt' in arguments")
	}
	if params.AIProvider == "" {
		params.AIProvider = "gemini" // Default fallback
	}
	return params, nil
}

// Execute performs the chat operation.
func (t *ChatTool) Execute(ctx context.Context, params ChatParams) (agent_domain.Result, error) {
	llm := t.llmRegistry.Get(params.AIProvider)
	if llm == nil {
		return agent_domain.Result{
			Success: false,
			Error:   "provider not found",
		}, types.Newf(types.ErrCodeNotFound, "LLM provider '%s' not found", params.AIProvider)
	}

	sysPrompt := params.SystemPrompt
	if t.schemaProvider != nil {
		// Embed tool schemas in the prompt
		schemas := t.schemaProvider.GetToolSchemas(nil)
		
		// Filter out the 'chat' tool to prevent infinite recursion
		var filtered []agent_domain.ToolSchema
		for _, s := range schemas {
			if s.Name != "chat" {
				filtered = append(filtered, s)
			}
		}

		if len(filtered) > 0 {
			toolsJSON, _ := json.MarshalIndent(filtered, "", "  ")
			sysPrompt += fmt.Sprintf(`

You have access to the following tools:
%s

You MUST respond in JSON with one of two formats:

1. To call a tool:
{"type": "tool_call", "tool_call": {"name": "tool_name", "args": {"key": "value"}}}

2. To provide your final answer:
{"type": "final_answer", "answer": "Your complete response here"}

Always respond with valid JSON. Do not include any text outside the JSON object.`, string(toolsJSON))
		}
	}

	messages := make([]domain.Message, 0, 10)
	if sysPrompt != "" {
		messages = append(messages, domain.Message{Role: domain.RoleSystem, Content: sysPrompt})
	}
	if params.AssistantPrefix != "" {
		messages = append(messages, domain.Message{Role: domain.RoleAssistant, Content: params.AssistantPrefix})
	}
	messages = append(messages, domain.Message{Role: domain.RoleUser, Content: params.Prompt})

	const maxSteps = 30
	var lastUsage domain.TokenUsage
	var finalResponse string

	// Helper for real-time logs
	emit := func(msg string, evtType subagent.EventType) {
		if e, ok := ctx.(interface{ Emit(any) }); ok {
			e.Emit(subagent.SubagentEvent{
				SessionID: "chat",
				Type:      evtType,
				Message:   msg,
				Timestamp: time.Now(),
			})
		}
	}

	emit("Thinking: Analyzing your request...", subagent.EventThought)

	for i := 0; i < maxSteps; i++ {
		var result domain.GenerationResult
		var err error
		
		// Use streaming for real-time responsiveness
		maxRetries := 3
		var streamChan <-chan domain.ChatChunk
		for retry := 0; retry <= maxRetries; retry++ {
			streamChan, err = llm.Stream(ctx, messages, nil)
			if err == nil {
				break
			}
			
			// If it's a 429 Too Many Requests, wait and retry with exponential backoff
			if strings.Contains(err.Error(), "429") && retry < maxRetries {
				backoff := time.Duration(1<<(retry+2)) * time.Second // Start with 4s, then 8s, then 16s
				emit(fmt.Sprintf("Provider '%s' returned 429 (Too Many Requests). Backing off for %v...", llm.Name(), backoff), subagent.EventError)
				
				select {
				case <-ctx.Done():
					return agent_domain.Result{Success: false, Error: "context cancelled"}, ctx.Err()
				case <-time.After(backoff):
					continue
				}
			}
			break
		}
		
		var accumulatedStr strings.Builder

		if err == nil {
			// Phase 1 of response handling: stream chunks back to UI live
			for chunk := range streamChan {
				if chunk.Error != nil {
					err = chunk.Error
					// Only print error chunks if they're actual errors, EOF check should be in Stream provider logic usually
					break // For stream we abort reading
				}
				
				if chunk.Content != "" {
					accumulatedStr.WriteString(chunk.Content)
					
					// Real-time streaming for the LLM text output!
					emit(chunk.Content, subagent.EventStreamToken)
				}
				
				if chunk.Done {
					result.Usage = chunk.Usage
					break
				}
			}
			result.Content = accumulatedStr.String()
		}
		if err != nil {
			// Phase 6 Enhancements: Robust Multi-Provider Rotation
			providers := t.llmRegistry.List()
			
			// Track which providers we've already tried in this step to avoid infinite loops
			tried := make(map[string]bool)
			tried[llm.Name()] = true
			
			success := false
			for _, pName := range providers {
				if tried[pName] {
					continue
				}
				
				fallbackLLM := t.llmRegistry.Get(pName)
				if fallbackLLM == nil {
					continue
				}
				
				emit(fmt.Sprintf("Provider '%s' failed (%v). Rotating to fallback '%s'...", llm.Name(), err, pName), subagent.EventError)
				
				streamChan, err = fallbackLLM.Stream(ctx, messages, nil)
				if err == nil {
					accumulatedStr.Reset()
					for chunk := range streamChan {
						if chunk.Error != nil {
							err = chunk.Error
							break // Drop this fallback, maybe next will work
						}
						
						if chunk.Content != "" {
							accumulatedStr.WriteString(chunk.Content)
							emit(chunk.Content, subagent.EventStreamToken)
						}
						
						if chunk.Done {
							result.Usage = chunk.Usage
							break
						}
					}
					
					if err == nil {
						result.Content = accumulatedStr.String()
						llm = fallbackLLM
						success = true
						break
					}
				}
				tried[pName] = true
			}
			
			if !success {
				return agent_domain.Result{
					Success: false,
					Error:   err.Error(),
				}, types.Wrapf(err, types.ErrCodeInternal, "all LLM providers failed to generate response")
			}
		}

		lastUsage = result.Usage
		response := result.Content

		// Try to extract JSON if the LLM prefaces it with text or responds with JSONL (multiple objects)
		jsonBody := tryExtractFirstJSON(response)

		// Try parsing as JSON subagent-style response
		var parsed struct {
			Type     string `json:"type"`
			ToolCall *struct {
				Name string                 `json:"name"`
				Args map[string]interface{} `json:"args"`
			} `json:"tool_call,omitempty"`
			Answer string `json:"answer,omitempty"`
		}

		err = json.Unmarshal([]byte(jsonBody), &parsed)
		if err != nil {
			// If we couldn't parse JSON at all, treat the original response as final plain answer
			finalResponse = response
			break
		}

		if parsed.Type == "final_answer" {
			emit("Thinking: Formulating final response...", subagent.EventThought)
			finalResponse = parsed.Answer
			break
		}

		if parsed.Type == "tool_call" && parsed.ToolCall != nil && t.executor != nil {
			tc := parsed.ToolCall
			emit(fmt.Sprintf("Tool Call: Executing %s...", tc.Name), subagent.EventToolCall)
			
			// Infer capabilities based on tool name to trigger HITL approval natively
			reqCaps := []security.Capability{security.CapNetOutbound} // Default to safe-ish outbound network call instead of modify infra
			lowerName := strings.ToLower(tc.Name)
			if strings.Contains(lowerName, "read") || strings.Contains(lowerName, "search") || strings.Contains(lowerName, "find") || strings.Contains(lowerName, "list") || strings.Contains(lowerName, "get") {
				reqCaps = []security.Capability{security.CapReadFS}
			} else if lowerName == "shell" || lowerName == "terminal" || strings.Contains(lowerName, "bash") || strings.Contains(lowerName, "exec") {
				reqCaps = []security.Capability{security.CapExecuteShell}
			} else if strings.Contains(lowerName, "write") || strings.Contains(lowerName, "edit") || strings.Contains(lowerName, "modify") || strings.Contains(lowerName, "update") {
				reqCaps = []security.Capability{security.CapWriteFS}
			}

			// Execute the tool
			task := agent_domain.Task{
				ID:           fmt.Sprintf("chat_loop_%d", i),
				Tool:         tc.Name,
				Args:         tc.Args,
				RequiredCaps: reqCaps,
			}
			
			execRes, execErr := t.executor.Execute(ctx, task)
			
			// Use the scrubbed or extracted JSON for history to keep it clean, 
			// but if there was text before, maybe we want to keep it? 
			// For Phase 6, we keep it in history but ensure user doesn't see it if it's intermediate.
			messages = append(messages, domain.Message{
				Role: domain.RoleAssistant, Content: jsonBody,
			})

			if execErr != nil {
				messages = append(messages, domain.Message{
					Role: domain.RoleUser, Content: fmt.Sprintf("Tool '%s' failed: %v", tc.Name, execErr),
				})
			} else {
				resultJSON, _ := json.Marshal(execRes.Data)
				messages = append(messages, domain.Message{
					Role: domain.RoleUser, Content: fmt.Sprintf("Tool '%s' returned: %s", tc.Name, string(resultJSON)),
				})
			}
			continue
		}

		// Fallback if structured but unknown type
		finalResponse = response
		break
	}

	if finalResponse == "" {
		finalResponse = "Error: Agent loops exceeded maximum allowance without a final answer."
	}

	return agent_domain.Result{
		Status:  "success",
		Success: true,
		Data: map[string]interface{}{
			"response": finalResponse,
			"usage":    lastUsage,
			"provider": params.AIProvider,
		},
	}, nil
}

// tryExtractFirstJSON attempts to find the first complete JSON block in a string.
// Crucial for handling LLMs that output JSONL (multiple consecutive JSON objects).
func tryExtractFirstJSON(input string) string {
	input = strings.TrimSpace(input)

	// Look for the first '{'
	start := strings.Index(input, "{")
	if start == -1 {
		return input
	}

	// Use json.NewDecoder to safely decode exactly one JSON object
	dec := json.NewDecoder(strings.NewReader(input[start:]))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err == nil {
		return string(raw) // This returns EXACTLY the first parsed JSON object
	}

	// Fallback to old heuristic if decoder fails
	end := strings.LastIndex(input, "}")
	if start != -1 && end != -1 && end > start {
		return input[start : end+1]
	}

	return input
}

