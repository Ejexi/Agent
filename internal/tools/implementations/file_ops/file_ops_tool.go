package file_ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/tools/base"
	"github.com/SecDuckOps/agent/internal/tools/implementations/filesystem"
	"github.com/SecDuckOps/shared/types"
)

type FileOpsParams struct {
	Action          string `json:"action"` // "view", "replace", "create"
	FilePath        string `json:"file_path"`
	TargetText      string `json:"target_text,omitempty"`
	ReplacementText string `json:"replacement_text,omitempty"`
	Content         string `json:"content,omitempty"`
}

type FileOpsTool struct {
	base.BaseTypedTool[FileOpsParams]
	gate *filesystem.WardenGate
}

func NewFileOpsTool(gate *filesystem.WardenGate) *FileOpsTool {
	t := &FileOpsTool{
		gate: gate,
	}
	t.Impl = t
	return t
}

func (t *FileOpsTool) Name() string {
	return "file_edit"
}

func (t *FileOpsTool) Schema() agent_domain.ToolSchema {
	return agent_domain.ToolSchema{
		Name:        "file_edit",
		Description: "A native tool to safely view, edit (search-and-replace), or create files. Use this instead of running raw bash 'sed' or 'cat' commands to avoid escaping syntax errors.",
		Parameters: map[string]string{
			"action":           "string (required: 'view', 'replace', 'create')",
			"file_path":        "string (required: absolute or relative path to the file)",
			"target_text":      "string (required for 'replace': the exact text to be replaced)",
			"replacement_text": "string (required for 'replace': the new text to insert)",
			"content":          "string (required for 'create': the full content of the new file)",
		},
	}
}

func (t *FileOpsTool) ParseParams(input map[string]interface{}) (FileOpsParams, error) {
	params, err := base.DefaultParseParams[FileOpsParams](input)
	if err != nil {
		return params, err
	}

	validActions := map[string]bool{"view": true, "replace": true, "create": true}
	if !validActions[params.Action] {
		return params, types.Newf(types.ErrCodeInvalidInput, "invalid action: %s. Must be 'view', 'replace', or 'create'", params.Action)
	}

	if params.FilePath == "" {
		return params, types.New(types.ErrCodeInvalidInput, "file_path is required")
	}

	if params.Action == "replace" && params.TargetText == "" {
		return params, types.New(types.ErrCodeInvalidInput, "target_text is required for replacing file content")
	}

	if params.Action == "create" && params.Content == "" {
		return params, types.New(types.ErrCodeInvalidInput, "content is required for creating a file")
	}

	return params, nil
}

func (t *FileOpsTool) Execute(ctx context.Context, params FileOpsParams) (agent_domain.Result, error) {
	// Security / Sanity check - absolute path resolution
	absPath, err := filepath.Abs(params.FilePath)
	if err != nil {
		return agent_domain.Result{Success: false, Error: fmt.Sprintf("failed to resolve absolute path: %v", err)}, nil
	}

	switch params.Action {
	case "view":
		if t.gate != nil {
			if err := t.gate.CheckRead(ctx, absPath); err != nil {
				return agent_domain.Result{Success: false, Error: err.Error()}, nil
			}
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			return agent_domain.Result{Success: false, Error: fmt.Sprintf("failed to read file: %v", err)}, nil
		}
		// Truncate if massive
		content := string(data)
		if len(content) > 10000 {
			content = content[:10000] + "...(truncated for length)"
		}
		return domain.Result{
			Success: true,
			Data:    map[string]interface{}{"content": content},
		}, nil

	case "create":
		if t.gate != nil {
			if err := t.gate.CheckWrite(ctx, absPath); err != nil {
				return domain.Result{Success: false, Error: err.Error()}, nil
			}
		}
		// Ensure directory exists
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return domain.Result{Success: false, Error: fmt.Sprintf("failed to create directories: %v", err)}, nil
		}

		err := os.WriteFile(absPath, []byte(params.Content), 0644)
		if err != nil {
			return domain.Result{Success: false, Error: fmt.Sprintf("failed to create file: %v", err)}, nil
		}
		return domain.Result{
			Success: true,
			Data:    map[string]interface{}{"message": fmt.Sprintf("File created successfully at %s", absPath)},
		}, nil

	case "replace":
		if t.gate != nil {
			if err := t.gate.CheckWrite(ctx, absPath); err != nil {
				return domain.Result{Success: false, Error: err.Error()}, nil
			}
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			return domain.Result{Success: false, Error: fmt.Sprintf("failed to read file for replacement: %v", err)}, nil
		}

		content := string(data)
		if !strings.Contains(content, params.TargetText) {
			return domain.Result{Success: false, Error: "the target_text was not found in the file exactly as provided"}, nil
		}

		newContent := strings.Replace(content, params.TargetText, params.ReplacementText, 1)

		err = os.WriteFile(absPath, []byte(newContent), 0644)
		if err != nil {
			return domain.Result{Success: false, Error: fmt.Sprintf("failed to write updated file: %v", err)}, nil
		}

		return domain.Result{
			Success: true,
			Data:    map[string]interface{}{"message": fmt.Sprintf("Successfully replaced text in %s", absPath)},
		}, nil
	}

	return domain.Result{Success: false, Error: "Unknown action"}, nil
}
