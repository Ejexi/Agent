package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
	sa "github.com/SecDuckOps/agent/internal/domain/subagent"
	"github.com/SecDuckOps/agent/internal/ports"
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
// Downgrades expensive models to cheaper ones (e.g., opus → haiku, sonnet → haiku).
func downgradeModel(model string) string {
	if model == "" {
		return ""
	}
	lower := strings.ToLower(model)

	if strings.Contains(lower, "opus") {
		return strings.Replace(model, "opus", "haiku", 1)
	}
	if strings.Contains(lower, "sonnet") {
		return strings.Replace(model, "sonnet", "haiku", 1)
	}
	// GPT-4 → GPT-3.5
	if strings.Contains(lower, "gpt-4") {
		return "gpt-3.5-turbo"
	}
	return model
}

// buildSystemPrompt creates the system prompt with 4-tuple context.
func (a *SessionActor) buildSystemPrompt() string {
	config := a.session.Subagent.Config

	// Build base prompt
	var prompt strings.Builder
	prompt.WriteString("You are Duckops, an expert DevOps Agent running in a terminal interface. You have deep knowledge of cloud infrastructure, CI/CD, automation, monitoring, and system reliability. Your role is to analyze problems, think through solutions, research technology documentation, and help users solve their problems efficiently within the constraints of a command-line environment.\n\n")
	
	prompt.WriteString("# Core Principles\n")
	prompt.WriteString("- Analyze the problem thoroughly before proposing solutions\n")
	prompt.WriteString("- Do your research properly in official docs when in doubt or when asked about recent or fresh information\n")
	prompt.WriteString("- Document all generated values and important configuration details\n")
	prompt.WriteString("- Avoid assumptions - always confirm critical decisions with the user\n")
	prompt.WriteString("- Consider security, scalability, and maintainability in all solutions\n\n")

	prompt.WriteString("# Communication Style - TERMINAL OPTIMIZED\n")
	prompt.WriteString("You are running in a terminal interface with a senior dev personality:\n")
	prompt.WriteString("- Pragmatic and action-oriented - cut the fluff, get to work\n")
	prompt.WriteString("- Casual but competent - like that senior dev who actually knows their stuff\n")
	prompt.WriteString("- Solution-focused - less ceremony, more results\n")
	prompt.WriteString("- Skip the robotic 'I will now...' phrases\n")
	prompt.WriteString("- Just start doing: 'Checking cluster status...'\n")
	prompt.WriteString("- Use standard GitHub-style markdown. Functional symbols OK (✓✗⚠) but avoid decorative emojis.\n\n")

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
	prefix := fmt.Sprintf("\033[35m[SUBAGENT %s | %s]\033[0m ", a.session.Subagent.SessionID[:8], strings.ToUpper(string(role)))
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("\n%s%s\n", prefix, msg)
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
		{Role: shared_domain.RoleUser, Content: config.Instructions},
	}

	maxSteps := config.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 30
	}

	a.session.Emit(sa.SubagentEvent{
		Type:    sa.EventLog,
		Message: fmt.Sprintf("Starting agent loop — provider: %s, max_steps: %d, pause_on_approval: %v", provider, maxSteps, config.PauseOnApproval),
	})
	a.logRole(shared_domain.RoleSystem, "Starting background loop (provider: %s, MaxSteps: %d)", provider, maxSteps)

	for i := 0; i < maxSteps; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		a.session.Emit(sa.SubagentEvent{
			Type:    sa.EventLog,
			Message: fmt.Sprintf("Step %d/%d", i+1, maxSteps),
		})

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
				a.session.Emit(sa.SubagentEvent{
					Type:    sa.EventLog,
					Message: "Detected and scrubbed secrets from prompt before LLM call",
				})
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
			a.logRole(shared_domain.RoleAssistant, "Final Answer: %s", parsed.Answer)
			return nil
		}

		// ===== Tool call =====
		if parsed.Type == "tool_call" && parsed.ToolCall != nil {
			tc := parsed.ToolCall
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
					// Scrub the response before appending to context (though it should be clean from LLM)
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
					continue
				}
			}

			// ===== Execute tool via Kernel =====
			task := domain.Task{
				ID:   tcID,
				Tool: tc.Name,
				Args: tc.Args,
			}

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
					Content: fmt.Sprintf("Tool '%s' returned: %s", tc.Name, string(resultJSON)),
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
