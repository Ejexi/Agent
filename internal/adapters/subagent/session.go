package subagent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
	sa "github.com/SecDuckOps/agent/internal/domain/subagent"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/agent/internal/skills"
	shared_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/types"
)

// LLMProvider is the interface for LLM access.
type LLMProvider interface {
	Get(name string) shared_domain.LLM
	List() []string
}

// agentResponse is the structured JSON format the LLM responds with.
type agentResponse struct {
	Type     string         `json:"type"`
	ToolCall *agentToolCall `json:"tool_call,omitempty"`
	Answer   string         `json:"answer,omitempty"`
}

type agentToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// SessionActor runs the LLM agent loop for a single subagent session.
type SessionActor struct {
	executor       ports.ToolExecutor
	schemaProvider ports.ToolSchemaProvider
	secretScanner  ports.SecretScannerPort
	session        *SubagentSession
}

func NewSessionActor(executor ports.ToolExecutor, schemaProvider ports.ToolSchemaProvider, secretScanner ports.SecretScannerPort, session *SubagentSession) *SessionActor {
	return &SessionActor{
		executor:       executor,
		schemaProvider: schemaProvider,
		secretScanner:  secretScanner,
		session:        session,
	}
}

// downgradeModel applies model cost optimization for subagents.
// Downgrades expensive models to cheaper ones using an explicit mapping.
func downgradeModel(model string) string {
	if model == "" {
		return ""
	}
	
	// Exact matches or known prefix mappings (B6 Fix)
	mapping := map[string]string{
		"claude-3-opus-20240229":      "claude-3-haiku-20240307",
		"claude-3-5-sonnet-20240620":  "claude-3-5-haiku-20241022",
		"claude-3-5-sonnet-20241022":  "claude-3-5-haiku-20241022",
		"gpt-4":                       "gpt-4o-mini",
		"gpt-4-turbo":                 "gpt-4o-mini",
		"gpt-4o":                      "gpt-4o-mini",
		"openrouter/anthropic/claude-3.5-sonnet": "openrouter/anthropic/claude-3.5-haiku",
		"openrouter/anthropic/claude-3-opus":     "openrouter/anthropic/claude-3-haiku",
	}

	if downgraded, ok := mapping[model]; ok {
		return downgraded
	}
	
	// For standard openrouter mappings matching the generic anthropic ones
	if strings.Contains(model, "openrouter/") {
		if strings.HasSuffix(model, "claude-3.5-sonnet") {
			return strings.Replace(model, "sonnet", "haiku", 1)
		}
		if strings.HasSuffix(model, "claude-3-opus") {
			return strings.Replace(model, "opus", "haiku", 1)
		}
	}
	
	return model
}

// buildSystemPrompt creates the system prompt with 4-tuple context.
func (a *SessionActor) buildSystemPrompt() string {
	config := a.session.Subagent.Config

	// Build base prompt
	var prompt strings.Builder
	prompt.WriteString(skills.DuckOpsRules)
	prompt.WriteString("\n")

	prompt.WriteString(`
=== NATURAL TERMINAL EXECUTION RULES ===
RULE: When the user message is a raw shell / terminal command or clearly formatted as a CLI instruction (examples include: ls, cd folder, cat file.txt, git status, docker ps, grep "error" logs.txt), you MUST NOT explain, discuss, or reinterpret the command.

Instead, you MUST:
1. Treat the entire user message as an executable command intent.
2. Immediately call the OS execution tool (terminal) with:
   - command = first token of the command
   - args = remaining tokens exactly as provided
   - cwd = current agent workspace
3. Return the execution result (stdout, stderr, exit code) back to the user.

Do NOT:
- Ask for confirmation unless the command is destructive or explicitly unsafe.
- Translate the command into natural language.
- Provide theoretical explanations about what the command does.
- Modify arguments unless required for OS compatibility policy.

If the command is a discovery/inspection command (e.g., ls, dir, pwd, git status, docker ps):
- You SHOULD provide a brief, high-level summary of the output (e.g., "Found 5 Go files and a Dockerfile").
- Mention anything unusual or important (e.g., "The .env file is missing").
- Keep it concise and professional.

If the command fails:
- Return the error output.
- Then optionally suggest a corrective command.

If the user message is NOT a raw command, continue normal conversational reasoning and tool-selection behavior.
========================================
`)

	// Inject context if provided (the "C" in the 4-tuple)
	if config.Context != "" {
		prompt.WriteString("\n=== CONTEXT (from previous work) ===\n")
		prompt.WriteString(config.Context)
		prompt.WriteString("\n")
	}

	// Depth awareness
	if a.session.Subagent.Depth > 0 {
		prompt.WriteString(fmt.Sprintf("\n=== HIERARCHY ===\nYou are a subagent running at depth %d. ", a.session.Subagent.Depth))
		if a.session.Subagent.Depth < 3 {
			prompt.WriteString("You can further delegate sub-tasks using the 'dynamic_subagent_task' tool if allowed.")
		} else {
			prompt.WriteString("You are at the maximum delegation depth. Do not attempt to spawn further subagents.")
		}
		prompt.WriteString("\n")
	}

	// Available tools
	schemas := a.schemaProvider.GetToolSchemas(config.AllowedTools)
	toolsJSON, _ := json.MarshalIndent(schemas, "", "  ")

	prompt.WriteString(fmt.Sprintf(`
You have access to the following tools:
%s

You MUST respond in JSON with one of two formats:

1. To call a tool:
{"type": "tool_call", "tool_call": {"name": "tool_name", "args": {"key": "value"}}}

2. To provide your final answer:
{"type": "final_answer", "answer": "Your complete response here"}

Always respond with valid JSON. Do not include any text outside the JSON object.`, string(toolsJSON)))

	return prompt.String()
}

func (a *SessionActor) logRole(role shared_domain.MessageRole, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	a.session.Emit(sa.SubagentEvent{
		Type:    sa.EventLog,
		Message: fmt.Sprintf("[%s] %s", strings.ToUpper(string(role)), msg),
	})
}

// Run executes the full agent loop with pause-on-approval support.
func (a *SessionActor) Run() error {
	ctx := a.session.Ctx
	config := a.session.Subagent.Config

	// ===== Resolve LLM provider =====
	llmProvider, ok := a.executor.(LLMProvider)
	if !ok {
		return types.New(types.ErrCodeInternal, "executor does not implement LLMProvider")
	}

	provider := config.Provider
	if provider == "" {
		providers := llmProvider.List()
		if len(providers) == 0 {
			return types.New(types.ErrCodeInternal, "no LLM providers available")
		}
		provider = providers[0]
	}

	llm := llmProvider.Get(provider)
	if llm == nil {
		return types.Newf(types.ErrCodeNotFound, "LLM provider not found: %s", provider)
	}

	// ===== Model downgrade for subagents =====
	model := config.Model
	if model != "" {
		model = downgradeModel(model)
		a.session.Emit(sa.SubagentEvent{
			Type:    sa.EventLog,
			Message: fmt.Sprintf("Using downgraded model: %s", model),
		})
	}

	// ===== Build conversation =====
	systemPrompt := a.buildSystemPrompt()
	messages := []shared_domain.Message{
		{Role: shared_domain.RoleSystem, Content: systemPrompt},
		{Role: shared_domain.RoleAssistant, Content: "Understood. I am DuckOps, an expert DevSecOps agent. I will follow your instructions precisely, avoid fluff, and prioritize action."},
		{Role: shared_domain.RoleUser, Content: config.Instructions},
	}

	maxSteps := config.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 30
	}

	// Memory compression threshold
	const compressionThreshold = 25
	const historyReserve = 10 // Last 10 messages to keep intact

	a.session.Emit(sa.SubagentEvent{
		Type:    sa.EventLog,
		Message: fmt.Sprintf("Starting agent loop — provider: %s, max_steps: %d, pause_on_approval: %v", provider, maxSteps, config.PauseOnApproval),
	})
	a.logRole(shared_domain.RoleSystem, "Starting background loop (provider: %s, MaxSteps: %d)", provider, maxSteps)

	// Initialization before loop
	var lastToolName string
	var lastToolArgsHash string
	var identicalCallCount int

	for i := 0; i < maxSteps; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		a.session.Emit(sa.SubagentEvent{
			Type:    sa.EventLog,
			Message: fmt.Sprintf("Step %d/%d (State: Running)", i+1, maxSteps),
		})

		// ===== Memory Compression Check =====
		if len(messages) >= compressionThreshold {
			a.session.Emit(sa.SubagentEvent{
				Type:    sa.EventLog,
				Message: "Memory compression triggered: summarizing intermediate conversation history...",
			})
			compressed, err := a.compressHistory(ctx, llm, messages, historyReserve)
			if err != nil {
				a.session.Emit(sa.SubagentEvent{
					Type:    sa.EventError,
					Message: fmt.Sprintf("Memory compression failed: %v. Continuing without compression.", err),
				})
			} else {
				messages = compressed
				a.session.Emit(sa.SubagentEvent{
					Type:    sa.EventLog,
					Message: fmt.Sprintf("Memory compression successful. Current context size: %d messages.", len(messages)),
				})
			}
		}

		// ===== Scrub Secrets Before LLM =====
		promptText := "No prompt generated yet"
		if len(messages) > 0 {
			promptText = messages[len(messages)-1].Content
		}

		var pm security.PlaceholderMap
		if a.secretScanner != nil {
			var scrubbed string
			scrubbed, pm = a.secretScanner.Scrub(a.session.Subagent.SessionID, promptText)
			if scrubbed != promptText {
				messages[len(messages)-1].Content = scrubbed
				// Less noisy logging internally
			}
		}

		// ===== Call LLM =====
		result, err := llm.Generate(ctx, messages, nil)
		if err != nil {
			return types.Wrapf(err, types.ErrCodeInternal, "LLM call failed on step %d", i+1)
		}

		response := result.Content

		// ===== Restore Secrets After LLM =====
		if a.secretScanner != nil && len(pm.Mappings) > 0 {
			response = a.secretScanner.Restore(response, pm)
		}

		// ===== Parse response =====
		var parsed agentResponse
		if err := json.Unmarshal([]byte(response), &parsed); err != nil {
			// Non-JSON = final answer
			a.session.mu.Lock()
			a.session.Subagent.Result = response
			a.session.mu.Unlock()
			a.session.Emit(sa.SubagentEvent{Type: sa.EventResult, Message: response})
			a.logRole(shared_domain.RoleAssistant, "Final Answer: %s", response)
			return nil
		}

		// ===== Final answer =====
		if parsed.Type == "final_answer" {
			a.session.mu.Lock()
			a.session.Subagent.Result = parsed.Answer
			a.session.mu.Unlock()
			a.session.Emit(sa.SubagentEvent{Type: sa.EventResult, Message: parsed.Answer})
			
			a.session.Emit(sa.SubagentEvent{
				Type:    sa.EventLog,
				Message: "Lifecycle: Finished → Task execution completed naturally",
			})

			a.logRole(shared_domain.RoleAssistant, "Final Answer: %s", parsed.Answer)
			return nil
		}

		// ===== Tool call =====
		if parsed.Type == "tool_call" && parsed.ToolCall != nil {
			tc := parsed.ToolCall
			
			// Compute simple hash for arguments
			argsBytes, _ := json.Marshal(tc.Args)
			currentArgsHash := string(argsBytes)

			// Termination Guard: Exactly repeating the last tool call
			if tc.Name == lastToolName && currentArgsHash == lastToolArgsHash {
				identicalCallCount++
				
				// First offense: Warn the LLM strongly
				if identicalCallCount == 1 {
					a.session.Emit(sa.SubagentEvent{
						Type:    sa.EventLog,
						Message: fmt.Sprintf("Guard triggered: Recursive loop detected. Subagent repeatedly calling '%s'. Forcing termination sequence.", tc.Name),
					})
					
					messages = append(messages, shared_domain.Message{
						Role: shared_domain.RoleAssistant, Content: response,
					})
					messages = append(messages, shared_domain.Message{
						Role:    shared_domain.RoleUser,
						Content: "SYSTEM GUARDFENCE: You have executed the exact same tool with the exact same arguments consecutively. This indicates an infinite loop. DO NOT REPEAT THIS TOOL CALL. You MUST immediately generate a 'final_answer' summarizing your findings so far, or your process will be structurally terminated.",
					})
					continue
				}

				// Second offense: Kill the agent completely
				a.session.Emit(sa.SubagentEvent{
					Type:    sa.EventError,
					Message: fmt.Sprintf("Fatal Guardfence: Subagent stubbornly repeated '%s' consecutive times despite warnings. Terminating.", tc.Name),
				})
				return types.Newf(types.ErrCodeExecutionFailed, "agent gracefully killed: infinite loop identified on tool '%s'", tc.Name)
			}

			// Update loop memory
			lastToolName = tc.Name
			lastToolArgsHash = currentArgsHash
			identicalCallCount = 0

			tcID := fmt.Sprintf("tc_%s_%d", a.session.Subagent.SessionID[:8], i)

			a.session.Emit(sa.SubagentEvent{
				Type:    sa.EventToolCall,
				Message: fmt.Sprintf("Tool call: %s", tc.Name),
				Data: map[string]interface{}{
					"id":   tcID,
					"tool": tc.Name,
					"args": tc.Args,
				},
			})
			a.logRole(shared_domain.RoleAssistant, "Calling tool: %s", tc.Name)

			// ===== Pause-on-approval (non-sandbox mode) =====
			if config.PauseOnApproval {
				approved, err := a.waitForApproval(ctx, tcID, tc)
				if err != nil {
					return err
				}

				if !approved {
					// Tool rejected — inform LLM
					cleanResponse := response
					if a.secretScanner != nil {
						cleanResponse, _ = a.secretScanner.Scrub(a.session.Subagent.SessionID, response)
					}
					messages = append(messages, shared_domain.Message{
						Role: shared_domain.RoleAssistant, Content: cleanResponse,
					})
					messages = append(messages, shared_domain.Message{
						Role:    shared_domain.RoleUser,
						Content: fmt.Sprintf("Tool '%s' was REJECTED by the master agent. Try a different approach.", tc.Name),
					})
					
					// Rejections reset loop memory so they can retry tools safely
					lastToolName = ""
					continue
				}
			}

			// ===== Execute tool via Kernel =====
			task := domain.Task{
				ID:        tcID,
				SessionID: a.session.Subagent.SessionID,
				Tool:      tc.Name,
				Args:      tc.Args,
			}

			a.session.Emit(sa.SubagentEvent{
				Type:    sa.EventLog,
				Message: fmt.Sprintf("Executing tool '%s' via sandbox boundary (timeout: configured per tool)", tc.Name),
			})

			result, execErr := a.executor.Execute(ctx, task)

			messages = append(messages, shared_domain.Message{
				Role: shared_domain.RoleAssistant, Content: response,
			})

			if execErr != nil {
				messages = append(messages, shared_domain.Message{
					Role:    shared_domain.RoleUser,
					Content: fmt.Sprintf("Tool '%s' failed: %v", tc.Name, execErr),
				})
				a.session.Emit(sa.SubagentEvent{
					Type:    sa.EventError,
					Message: fmt.Sprintf("Tool '%s' failed: %v", tc.Name, execErr),
				})
				a.logRole(shared_domain.RoleTool, "Tool error: %v", execErr)
			} else {
				resultJSON, _ := json.Marshal(result.Data)
				messages = append(messages, shared_domain.Message{
					Role:    shared_domain.RoleUser,
					Content: fmt.Sprintf("Tool '%s' returned: %s\n\nSYSTEM_NOTE: Evaluate the output. If the task is strictly fulfilled, emit 'final_answer'.", tc.Name, string(resultJSON)),
				})
				a.logRole(shared_domain.RoleTool, "Tool output received (length: %d)", len(resultJSON))
			}
			continue
		}

		// Unknown type = final answer
		a.session.mu.Lock()
		a.session.Subagent.Result = response
		a.session.mu.Unlock()
		a.session.Emit(sa.SubagentEvent{Type: sa.EventResult, Message: response})
		return nil
	}

	return types.Newf(types.ErrCodeInternal, "agent loop exceeded maximum steps (%d)", maxSteps)
}

// waitForApproval pauses the session and waits for the master agent's approval.
// Returns true if approved, false if rejected.
func (a *SessionActor) waitForApproval(ctx context.Context, tcID string, tc *agentToolCall) (bool, error) {
	pauseInfo := &sa.PauseInfo{
		Reason:  sa.PauseToolApproval,
		Message: fmt.Sprintf("Tool '%s' requires approval", tc.Name),
		PendingToolCalls: []sa.PendingToolCall{
			{ID: tcID, Name: tc.Name, Args: tc.Args},
		},
	}

	a.session.SetPauseInfo(pauseInfo)

	a.session.Emit(sa.SubagentEvent{
		Type:    sa.EventLog,
		Message: fmt.Sprintf("⏸ Paused — waiting for approval on tool '%s' (id: %s)", tc.Name, tcID),
	})

	// Block until we receive a resume decision or context cancellation
	select {
	case <-ctx.Done():
		return false, ctx.Err()

	case decision := <-a.session.ResumeChan:
		// Clear pause info
		a.session.mu.Lock()
		a.session.Subagent.PauseInfo = nil
		a.session.mu.Unlock()

		a.session.SetStatus(sa.StatusRunning)
		a.session.Emit(sa.SubagentEvent{
			Type:    sa.EventResumed,
			Message: "Resumed by master agent",
		})

		// Determine if approved
		if decision.ApproveAll {
			return true, nil
		}
		if decision.RejectAll {
			return false, nil
		}

		// Check specific approvals
		for _, approvedID := range decision.Approve {
			if approvedID == tcID {
				return true, nil
			}
		}

		// If not explicitly approved, it's rejected
		return false, nil
	}
}

// compressHistory reduces message count by summarizing old context while preserving recent state.
func (a *SessionActor) compressHistory(ctx context.Context, llm shared_domain.LLM, messages []shared_domain.Message, reserve int) ([]shared_domain.Message, error) {
	if len(messages) <= reserve+3 {
		return messages, nil
	}

	// 1. Keep the core setup (System, Ack, Instruction)
	head := messages[0:3]

	// 2. Extract context to summarize (the "middle")
	midIdx := len(messages) - reserve
	middle := messages[3:midIdx]
	tail := messages[midIdx:]

	// 3. Build summary prompt
	var summaryText strings.Builder
	for _, m := range middle {
		summaryText.WriteString(fmt.Sprintf("%s: %s\n", strings.ToUpper(string(m.Role)), m.Content))
	}

	summaryMessages := []shared_domain.Message{
		{
			Role:    shared_domain.RoleSystem,
			Content: "You are a specialized summarization engine. Summarize the following agent conversation history into a concise, few-paragraph narrative that preserves all key findings, tool results, and the current operational state. Focus on 'What has been done' and 'What was found'.",
		},
		{
			Role:    shared_domain.RoleUser,
			Content: fmt.Sprintf("CONVERSATION TO SUMMARIZE:\n%s", summaryText.String()),
		},
	}

	// 4. Call LLM for summary
	result, err := llm.Generate(ctx, summaryMessages, nil)
	if err != nil {
		return nil, err
	}

	// 5. Reconstruct messages: [head..., summary, tail...]
	newMessages := make([]shared_domain.Message, 0, 3+1+len(tail))
	newMessages = append(newMessages, head...)
	newMessages = append(newMessages, shared_domain.Message{
		Role:    shared_domain.RoleSystem,
		Content: "=== PREVIOUS HISTORY SUMMARY ===\n" + result.Content + "\n================================",
	})
	newMessages = append(newMessages, tail...)

	return newMessages, nil
}
