package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/ports"
	types "github.com/SecDuckOps/shared/types"
)

// Runtime handles the execution of tools.
type Runtime struct {
	registry   ports.ToolRegistry
	auditLog   ports.AuditLogPort
	classifier ports.ThinkingPort
}

// NewRuntime creates a new runtime.
func NewRuntime(registry ports.ToolRegistry, auditLog ports.AuditLogPort, classifier ports.ThinkingPort) *Runtime {
	return &Runtime{
		registry:   registry,
		auditLog:   auditLog,
		classifier: classifier,
	}
}

// Execute runs a tool based on the provided task.
func (r *Runtime) Execute(ctx *ExecutionContext, task domain.Task) (domain.Result, error) {
	if r.registry == nil {
		err := types.New(types.ErrCodeInternal, "runtime registry is not initialized")
		return domain.Result{
			TaskID:  task.ID,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	tool, err := r.registry.GetTool(ctx, task.Tool)
	if err != nil {
		appErr := types.Newf(types.ErrCodeToolNotFound, "tool not found: %s", task.Tool)
		return domain.Result{
			TaskID:  task.ID,
			Success: false,
			Error:   appErr.Error(),
		}, appErr
	}

	// Security Policy Enforcement: Verify task capability requirements
	if !ctx.HasCapabilities(task.RequiredCaps) {
		err := types.Newf(types.ErrCodePermissionDenied, "security denial: insufficient capabilities for task %s", task.ID)
		
		// Log the denial
		if r.auditLog != nil {
			_ = r.auditLog.Record(ctx, security.AuditEntry{
				SessionID: task.SessionID,
				Action:    security.AuditPolicyDeny,
				Actor:     ctx.PrincipalID,
				Target:    task.Tool,
				Details: map[string]interface{}{
					"required_caps": task.RequiredCaps,
					"granted_caps":  ctx.GrantedCaps,
					"reason":        "capability mismatch",
				},
				Timestamp: time.Now(),
			})
		}

		return domain.Result{
			TaskID:  task.ID,
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// ── HITL: Human-In-The-Loop Approval ──
	// Require explicit user approval for any task that modifies state or runs commands.
	needsApproval := false
	
	// Only ask for approval if there is an interactive UI listening for events.
	// Trigger approval ONLY for highly destructive or sensitive capabilities.
	if ctx.OnEvent != nil {
		for _, cap := range task.RequiredCaps {
			if cap == security.CapExecuteShell || cap == security.CapModifyInfra || cap == security.CapAccessKubernetes || cap == security.CapAgentControl {
				// Special exception: ask the Smart Agent if this specific command is safe.
				// For example, if it's just 'ls' or 'git status', it shouldn't need user approval.
				isSafeShell := false
				if r.classifier != nil && (task.Tool == "shell" || task.Tool == "terminal") {
					cmd, _ := task.Args["command"].(string)
					
					var args []string
					switch v := task.Args["args"].(type) {
					case []interface{}:
						for _, a := range v {
							args = append(args, fmt.Sprint(a))
						}
					case []string:
						args = v
					}
					
					safe, err := r.classifier.IsSafeToAutoExecute(ctx, cmd, args)
					if err == nil && safe {
						isSafeShell = true
					}
				}
				
				if isSafeShell {
					continue // Bypass approval for this specific safe capability
				}

				needsApproval = true
				break
			}
		}
	}

	if needsApproval {
		var question string
		header := "Security Gate"

		if task.Tool == "shell" || task.Tool == "terminal" {
			header = "Terminal Exec"
			cmd, _ := task.Args["command"].(string)
			argsStr := ""
			if args, ok := task.Args["args"]; ok {
				argsStr = fmt.Sprintf("%v", args)
			}
			question = fmt.Sprintf("Allow agent to execute:\n  %s %s", cmd, argsStr)
			if cwd, ok := task.Args["cwd"].(string); ok && cwd != "" && cwd != "." {
				question += fmt.Sprintf("\n\nWorkspace: %s", cwd)
			}
		} else {
			header = "Tool Exec"
			question = fmt.Sprintf("Allow agent to run tool '%s'?\nParams: %v", task.Tool, task.Args)
		}

		askEvent := domain.AskUserEvent{
			Questions: []domain.AskUserQuestion{
				{
					Header:   header,
					Question: question,
					Type:     domain.QuestionTypeYesNo,
				},
			},
			ResponseChan: make(chan domain.AskUserResponse, 1),
		}

		ctx.Emit(askEvent)

		select {
		case resp := <-askEvent.ResponseChan:
			if resp.Cancelled || len(resp.Answers) == 0 || resp.Answers[0] != "yes" {
				err := types.New(types.ErrCodePermissionDenied, "user denied execution")
				return domain.Result{
					TaskID:  task.ID,
					Success: false,
					Error:   err.Error(),
				}, err
			}
		case <-time.After(10 * time.Minute):
			err := types.New(types.ErrCodePermissionDenied, "execution confirmation timed out")
			return domain.Result{
				TaskID:  task.ID,
				Success: false,
				Error:   err.Error(),
			}, err
		case <-ctx.Done():
			err := types.New(types.ErrCodeInternal, "context cancelled during approval")
			return domain.Result{
				TaskID:  task.ID,
				Success: false,
				Error:   err.Error(),
			}, err
		}
	}

	// 1. Audit Log: Tool Execution Started
	if r.auditLog != nil {
		_ = r.auditLog.Record(ctx, security.AuditEntry{
			SessionID: task.SessionID, // Requires SessionID on Task
			Action:    security.AuditToolExecute,
			Actor:     ctx.PrincipalID,
			Target:    task.Tool,
			Details: map[string]interface{}{
				"args": task.Args,
			},
			Timestamp: time.Now(),
		})
	}

	// Inject SessionID into standard context so tools don't need to import kernel packages
	toolCtx := context.WithValue(ctx, "sessionID", task.SessionID)

	// Runtime executes the tool (only the runtime should execute this)
	result, err := tool.ExecuteRaw(toolCtx, task.Args)
	if err != nil {
		appErr := types.Wrapf(err, types.ErrCodeToolExecution, "failed to execute tool %s", task.Tool)
		return result, appErr
	}

	// Ensure the result has the correct TaskID
	result.TaskID = task.ID

	// 2. Audit Log: Tool Execution Completed
	if r.auditLog != nil {
		_ = r.auditLog.Record(ctx, security.AuditEntry{
			SessionID: task.SessionID,
			Action:    security.AuditToolResult,
			Actor:     "kernel",
			Target:    task.Tool,
			Details: map[string]interface{}{
				"success": result.Success,
				"error":   result.Error,
			},
			Timestamp: time.Now(),
		})
	}

	return result, nil
}

// ExecuteBatch runs multiple tools sequentially to ensure predictable state manipulation.
// It acts as a queue: the next command waits for the previous one to finish.
func (r *Runtime) ExecuteBatch(ctx *ExecutionContext, tasks []domain.Task) ([]domain.Result, error) {
	if r.registry == nil {
		return nil, types.New(types.ErrCodeInternal, "runtime registry is not initialized")
	}

	results := make([]domain.Result, len(tasks))

	for i, task := range tasks {
		res, err := r.Execute(ctx, task)
		results[i] = res
		
		// Fast-fail: Stop executing the queue if a command fails
		if err != nil {
			return results, err
		}
		if !res.Success {
			// Stop execution on logical failure even if err is nil
			return results, types.Newf(types.ErrCodeToolExecution, "task %s failed", task.ID)
		}
	}

	return results, nil
}
