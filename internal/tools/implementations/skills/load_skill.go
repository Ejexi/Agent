package skills

import (
	"context"
	"fmt"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
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
	return agent_domain.ToolSchema{
		Name:        "load_skill",
		Description: "Dynamically loads a specialized skill (markdown documentation) into your context. Use this when you are asked about tools like `duckops autopilot`, `duckops board`, or when you need detailed guidance on subagents. You MUST load a skill before proposing implementations related to these topics.",
		Parameters: map[string]string{
			"skill_name": "string (the name of the skill to load, e.g., 'taskboard', 'autopilot', 'subagents')",
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

	// Returning a map because Agent expects domain.Result.Data to be map[string]interface{}
	return agent_domain.Result{
		Success: true,
		Data: map[string]interface{}{
			"content":     skill.Content,
			"description": skill.Description,
		},
	}, nil
}
