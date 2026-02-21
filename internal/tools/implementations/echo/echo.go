// testing tools
package echo

import (
	"context"
	"fmt"
	"time"

	"github.com/Ejexi/Agent/internal/tools/base"
)

// =======================  TYPED PARAMETERS =========================

// defines the typed parameters for echo tool
// input parameters of the tool
type Params struct {
	Message string `json:"message"`
}

// ======================= ECHO TOOL =========================

// simple tool that echoes back messages
// testing the tool system
type EchoTool struct {
	*base.BaseTool // Embed BaseTool to get Name() and Description()
}

// creates a new echo tool Constructor
func New() *EchoTool {
	return &EchoTool{ //return pointer object to use it latter
		BaseTool: base.NewBaseTool(
			"echo",
			"Echoes back the input message We Use this to test if tools are working",
		),
	}
}

//======================= TYPED EXECUTE TypeSafe =========================

// Execute runs the echo tool
func (t *EchoTool) Execute(ctx context.Context, params Params) (*base.ToolResult, error) {
	start := time.Now()

	// Use typed params directly
	result := fmt.Sprintf("Echo: %s", params.Message)

	// Return success result
	return &base.ToolResult{
		Success:  true,
		Data:     result,
		Error:    nil,
		Duration: time.Since(start),
	}, nil
}

// =======================  RAW EXECUTE For Registry =========================
// handles raw data used by agent/registry
// Converts raw data -> typed params -> calls Execute()
func (t *EchoTool) ExecuteRaw(ctx context.Context, data map[string]interface{}) (*base.ToolResult, error) {
	start := time.Now()

	// 1) Parse and validate
	params, err := t.ParseParams(data)
	if err != nil {
		return &base.ToolResult{
			Success:  false,
			Error:    err,
			Duration: time.Since(start),
		}, err
	}

	// 2) Call typed execute
	return t.Execute(ctx, params)
}

// =======================  PARSE & VALIDATE PARAMETERS =========================
// converts raw data to typed Params
// Validation done here
func (t *EchoTool) ParseParams(data map[string]interface{}) (Params, error) {

	// Get message from data
	msgRaw, exists := data["message"]
	if !exists {
		return Params{}, fmt.Errorf("missing required parameter: message")
	}

	//Type check
	msg, ok := msgRaw.(string)
	if !ok {
		return Params{}, fmt.Errorf("parameter 'message' must be a string, got %T", msgRaw)
	}

	//Validate
	if msg == "" {
		return Params{}, fmt.Errorf("parameter 'message' cannot be empty")
	}
	return Params{Message: msg}, nil
}

// ============================SCHEMA For LLM========================

// returns the tool definition for LLM function calling in Structured way
func (t *EchoTool) Schema() base.ToolSchema {
	return base.ToolSchema{
		Name:        "echo",
		Description: "Echoes back the input message",
		Parameters: base.ParameterSchema{
			Type: "object",
			Properties: map[string]base.PropertySchema{
				"message": {
					Type:        "string",
					Description: "The message to echo back",
				},
			},
			Required: []string{"message"},
		},
	}
}
