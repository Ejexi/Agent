package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/Ejexi/Agent/internal/agent/memory"
	"github.com/Ejexi/Agent/internal/core/logger"
	"github.com/Ejexi/Agent/internal/tools/registry"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"go.uber.org/zap"
)

//====================AGENT STRUCT========================

//  main brain that orchestrates everything

type Agent struct {
	client   *openai.Client     // connection to LLM (OpenRouter)
	registry *registry.Registry //The avilable tools
	memory   *memory.Memory     // conversation history
	model    string             //which LLM model to use
	logger   *logger.Logger     //for logger
}

// holds configuration for creating an agent
type Config struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	MaxTokens   int
}

//====================CONSTRUCTOR========================

// creates and returns a new Agent instance
func New(cfg Config, reg *registry.Registry, log *logger.Logger) *Agent {

	// Create OpenAI client work with any client
	client := openai.NewClient(
		option.WithBaseURL(cfg.BaseURL), //api endpoint
		option.WithAPIKey(cfg.APIKey),   //api key auth
	)
	return &Agent{
		client:   &client,        //client we just created
		registry: reg,            //tools registry
		memory:   memory.New(10), // placholder will be converted to token based system latter
		model:    cfg.Model,      // normal model name
		logger:   log,
	}
}

// ========================REQUEST HANDLER==========================

// main entry point for handling user input
// Flow: input -> memory -> LLM -> check tool -> execute tool -> response
func (a *Agent) ProcessRequest(ctx context.Context, userInput string) (string, error) {
	a.logger.Info("Processing request", zap.String("input", userInput))

	// 1) Add user message to memory
	a.memory.AddMessage(memory.Message{
		Role:    "user",
		Content: userInput,
	})

	// 2)  Build messages history + current message
	message := a.buildMessages()

	// 3) Call LLM with messages
	response, err := a.callLLM(ctx, message)
	if err != nil {
		a.logger.Error("LLM call failed", zap.Error(err))
		return "", fmt.Errorf("failed to get response from LLM: %w", err)
	}

	// Step 4: Check if LLM wants to use a tool
	//< If response starts with "TOOL:" -> execute the tool >
	if strings.HasPrefix(response, "TOOL:") {
		a.logger.Info("Tool requested", zap.String("command", response))

		toolResponse, err := a.executeTool(ctx, response)
		if err != nil {
			a.logger.Error("Tool execution failed", zap.Error(err))
			// Don't crash -> tell user tool failed
			response = fmt.Sprintf("Tool failed: %v", err)
		} else {
			response = toolResponse
		}
	}
	a.memory.AddMessage(memory.Message{
		Role:    "assistant",
		Content: response,
	})
	a.logger.Info("Request processed successfully")
	return response, nil
}
