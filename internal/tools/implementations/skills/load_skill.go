package skills

import (
	"context"
	"fmt"
	"time"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/kernel"
	"github.com/SecDuckOps/agent/internal/skills"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/types"
)

type LoadSkillParams struct {
	SkillName string `json:"skill_name"`
}

type LoadSkillTool struct {
	base.BaseTypedTool[LoadSkillParams]
	registry skills.Registry
}

func NewLoadSkillTool(registry skills.Registry) *LoadSkillTool {
	t := &LoadSkillTool{registry: registry}
	t.Impl = t
	return t
}

func (t *LoadSkillTool) Name() string {
	return "load_skill"
}

func (t *LoadSkillTool) Schema() agent_domain.ToolSchema {
	desc := "Dynamically loads a specialized skill (markdown documentation) into your context. "

	return agent_domain.ToolSchema{
		Name:        "load_skill",
		Description: desc,
		Parameters: map[string]string{
			"skill_name": "string (the exact name of the skill to load)",
		},
	}
}

func (t *LoadSkillTool) ParseParams(input map[string]interface{}) (LoadSkillParams, error) {
	params, err := base.DefaultParseParams[LoadSkillParams](input)
	if err != nil {
		return params, err
	}
	if params.SkillName == "" {
		return params, types.New(types.ErrCodeInvalidInput, "skill_name is required")
	}
	return params, nil
}

func (t *LoadSkillTool) Execute(ctx context.Context, params LoadSkillParams) (agent_domain.Result, error) {
	skill, err := t.registry.GetSkill(params.SkillName)
	if err != nil {
		available := t.registry.ListSkills()
		names := ""
		for _, s := range available {
			names += s.Name + ", "
		}
		return agent_domain.Result{
			Success: false,
			Error:   fmt.Sprintf("skill '%s' not found. Available skills: %s", params.SkillName, names),
		}, nil
	}

	// Emit consent event — TUI shows a brief confirmation before injecting skill
	if execCtx, ok := ctx.(*kernel.ExecutionContext); ok {
		responseChan := make(chan bool, 1)
		execCtx.Emit(agent_domain.SkillActivationEvent{
			SkillName:    skill.Name,
			Description:  skill.Description,
			SkillDir:     "", // embedded skills have no filesystem path
			ResponseChan: responseChan,
		})

		// Block until user approves or 30-second auto-approve (non-blocking for embedded skills)
		select {
		case approved := <-responseChan:
			if !approved {
				return agent_domain.Result{
					Success: false,
					Error:   fmt.Sprintf("skill '%s' activation denied by user", params.SkillName),
				}, nil
			}
		case <-time.After(30 * time.Second):
			// Auto-approve after timeout — embedded skills are low-risk
		case <-ctx.Done():
			return agent_domain.Result{
				Success: false,
				Error:   types.New(types.ErrCodeInternal, "load_skill: context cancelled").Error(),
			}, nil
		}
	}

	return agent_domain.Result{
		Success: true,
		Data: map[string]interface{}{
			"content":     skill.Content,
			"description": skill.Description,
		},
	}, nil
}
