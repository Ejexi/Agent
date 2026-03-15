package todo

import (
	"context"
	"fmt"
	"sync"
	"time"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/types"
)

var (
	globalTodoList []TodoItem
	todoMu         sync.Mutex
	todoCounter    int
)

type TodoItem struct {
	ID          int       `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // "pending", "completed"
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type TodoParams struct {
	Action      string `json:"action"` // "add", "complete", "list"
	Description string `json:"description,omitempty"`
	ID          int    `json:"id,omitempty"`
}

type TodoTool struct {
	base.BaseTypedTool[TodoParams]
}

func NewTodoTool() *TodoTool {
	t := &TodoTool{}
	t.Impl = t
	return t
}

func (t *TodoTool) Name() string {
	return "todo"
}

func (t *TodoTool) Schema() agent_domain.ToolSchema {
	return agent_domain.ToolSchema{
		Name:        "todo",
		Description: "A tool for the agent to manage its execution plan and keep track of pending tasks. Highly recommended for complex, multi-step operations to avoid hallucination.",
		Parameters: map[string]string{
			"action":      "string (required: 'add', 'complete', 'list')",
			"description": "string (the task description, required for 'add')",
			"id":          "integer (the task ID, required for 'complete')",
		},
	}
}

func (t *TodoTool) ParseParams(input map[string]interface{}) (TodoParams, error) {
	params, err := base.DefaultParseParams[TodoParams](input)
	if err != nil {
		return params, err
	}

	validActions := map[string]bool{"add": true, "complete": true, "list": true}
	if !validActions[params.Action] {
		return params, types.Newf(types.ErrCodeInvalidInput, "invalid action: %s. Must be 'add', 'complete', or 'list'", params.Action)
	}

	if params.Action == "add" && params.Description == "" {
		return params, types.New(types.ErrCodeInvalidInput, "description is required to add a task")
	}

	if params.Action == "complete" && params.ID <= 0 {
		return params, types.New(types.ErrCodeInvalidInput, "a valid ID is required to complete a task")
	}

	return params, nil
}

func (t *TodoTool) Execute(ctx context.Context, params TodoParams) (agent_domain.Result, error) {
	todoMu.Lock()
	defer todoMu.Unlock()

	switch params.Action {
	case "add":
		todoCounter++
		item := TodoItem{
			ID:          todoCounter,
			Description: params.Description,
			Status:      "pending",
			CreatedAt:   time.Now(),
		}
		globalTodoList = append(globalTodoList, item)
		return agent_domain.Result{
			Success: true,
			Data:    map[string]interface{}{"message": fmt.Sprintf("Todo added with ID: %d", item.ID)},
		}, nil

	case "complete":
		for i, item := range globalTodoList {
			if item.ID == params.ID {
				if item.Status == "completed" {
					return agent_domain.Result{Success: true, Data: map[string]interface{}{"message": fmt.Sprintf("Todo %d is already completed", params.ID)}}, nil
				}
				globalTodoList[i].Status = "completed"
				globalTodoList[i].CompletedAt = time.Now()
				return agent_domain.Result{
					Success: true,
					Data:    map[string]interface{}{"message": fmt.Sprintf("Todo %d marked as completed", params.ID)},
				}, nil
			}
		}
		return agent_domain.Result{Success: false, Error: fmt.Sprintf("Todo ID %d not found", params.ID)}, nil

	case "list":
		if len(globalTodoList) == 0 {
			return agent_domain.Result{Success: true, Data: map[string]interface{}{"message": "The todo list is empty."}}, nil
		}
		return agent_domain.Result{
			Success: true,
			Data:    map[string]interface{}{"todos": globalTodoList},
		}, nil
	}

	return agent_domain.Result{Success: false, Error: "Unknown action"}, nil
}
