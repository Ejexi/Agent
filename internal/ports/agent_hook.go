package ports

import "context"

// AgentHook is a pluggable observer that receives callbacks at key points in the
// agent loop. Implementations must be non-blocking and safe to call concurrently.
//
// Mirrors duckops agent-core/src/hooks.rs AgentHook trait.
// All methods have default no-op implementations — embed NoOpAgentHook to get them.
type AgentHook interface {
	// BeforeInference is called before each LLM Generate call.
	// Use for: request logging, rate monitoring, prompt auditing.
	BeforeInference(ctx context.Context, sessionID, runID string, turn int) error

	// AfterInference is called immediately after a successful LLM response.
	// Use for: token usage tracking, response logging.
	AfterInference(ctx context.Context, sessionID, runID string, turn int, promptTokens, completionTokens int) error

	// BeforeToolExecution is called just before a tool is dispatched.
	// Use for: audit logging, sandboxing decisions, pre-execution checks.
	BeforeToolExecution(ctx context.Context, sessionID, runID, toolCallID, toolName string) error

	// AfterToolExecution is called after a tool returns (success or error).
	// Use for: audit logging, result scrubbing, telemetry.
	AfterToolExecution(ctx context.Context, sessionID, runID, toolCallID, toolName string, isError bool) error

	// OnError is called when the agent loop exits with a non-nil error.
	// Use for: alerting, structured error logging, retry decisions.
	OnError(ctx context.Context, sessionID, runID string, err error) error
}

// NoOpAgentHook provides default no-op implementations for all AgentHook methods.
// Embed this in custom hooks to only override what you need.
type NoOpAgentHook struct{}

func (NoOpAgentHook) BeforeInference(_ context.Context, _, _ string, _ int) error {
	return nil
}

func (NoOpAgentHook) AfterInference(_ context.Context, _, _ string, _, _, _ int) error {
	return nil
}

func (NoOpAgentHook) BeforeToolExecution(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (NoOpAgentHook) AfterToolExecution(_ context.Context, _, _, _, _ string, _ bool) error {
	return nil
}

func (NoOpAgentHook) OnError(_ context.Context, _, _ string, _ error) error {
	return nil
}
