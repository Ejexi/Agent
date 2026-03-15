package notes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/shared/types"
)

// Global in-memory storage for notes (for the lifetime of the root session)
var (
	globalNotes sync.Map
)

type Note struct {
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NotesParams struct {
	Action  string   `json:"action"` // "add", "update", "view", "list", "delete"
	Key     string   `json:"key,omitempty"`
	Content string   `json:"content,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type NotesTool struct {
	base.BaseTypedTool[NotesParams]
}

func NewNotesTool() *NotesTool {
	t := &NotesTool{}
	t.Impl = t
	return t
}

func (t *NotesTool) Name() string {
	return "notes"
}

func (t *NotesTool) Schema() agent_domain.ToolSchema {
	return agent_domain.ToolSchema{
		Name:        "notes",
		Description: "A persistent key-value store for the agent to remember important findings, credentials, IP addresses, or state across different steps or subagents.",
		Parameters: map[string]string{
			"action":  "string (required: 'add', 'update', 'view', 'list', 'delete')",
			"key":     "string (required for all actions except 'list')",
			"content": "string (the note content, required for 'add' and 'update')",
			"tags":    "array of strings (optional, for categorization)",
		},
	}
}

func (t *NotesTool) ParseParams(input map[string]interface{}) (NotesParams, error) {
	params, err := base.DefaultParseParams[NotesParams](input)
	if err != nil {
		return params, err
	}

	validActions := map[string]bool{"add": true, "update": true, "view": true, "list": true, "delete": true}
	if !validActions[params.Action] {
		return params, types.Newf(types.ErrCodeInvalidInput, "invalid action: %s. Must be 'add', 'update', 'view', 'list', or 'delete'", params.Action)
	}

	if params.Action != "list" && params.Key == "" {
		return params, types.New(types.ErrCodeInvalidInput, "key is required for this action")
	}

	if (params.Action == "add" || params.Action == "update") && params.Content == "" {
		return params, types.New(types.ErrCodeInvalidInput, "content is required for adding or updating a note")
	}

	return params, nil
}

func (t *NotesTool) Execute(ctx context.Context, params NotesParams) (agent_domain.Result, error) {
	switch params.Action {
	case "add", "update":
		globalNotes.Store(params.Key, Note{
			Content:   params.Content,
			Tags:      params.Tags,
			UpdatedAt: time.Now(),
		})
		return agent_domain.Result{
			Success: true,
			Data:    map[string]interface{}{"message": fmt.Sprintf("Note '%s' saved successfully.", params.Key)},
		}, nil

	case "view":
		if val, ok := globalNotes.Load(params.Key); ok {
			note := val.(Note)
			return agent_domain.Result{
				Success: true,
				Data:    map[string]interface{}{"note": note},
			}, nil
		}
		return agent_domain.Result{Success: false, Error: "Note not found"}, nil

	case "delete":
		globalNotes.Delete(params.Key)
		return agent_domain.Result{
			Success: true,
			Data:    map[string]interface{}{"message": fmt.Sprintf("Note '%s' deleted successfully.", params.Key)},
		}, nil

	case "list":
		results := make(map[string]Note)
		globalNotes.Range(func(key, value interface{}) bool {
			k := key.(string)
			v := value.(Note)
			results[k] = v
			return true // continue iteration
		})
		
		if len(results) == 0 {
			return agent_domain.Result{Success: true, Data: map[string]interface{}{"message": "No notes found."}}, nil
		}
		
		// Optional basic tag filtering logic if needed later, but for now just return all
		if len(params.Tags) > 0 {
			filtered := make(map[string]Note)
			for k, v := range results {
				include := false
				for _, searchTag := range params.Tags {
					for _, noteTag := range v.Tags {
						if strings.EqualFold(searchTag, noteTag) {
							include = true
							break
						}
					}
					if include {
						filtered[k] = v
						break
					}
				}
			}
			return agent_domain.Result{Success: true, Data: map[string]interface{}{"notes": filtered}}, nil
		}

		return agent_domain.Result{Success: true, Data: map[string]interface{}{"notes": results}}, nil
	}

	return agent_domain.Result{Success: false, Error: "Unknown action"}, nil
}
