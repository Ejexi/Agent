package main

import (
	"context"
	"fmt"

	"github.com/Ejexi/Agent/internal/agent/memory"
	"github.com/Ejexi/Agent/internal/tools/base"
	"github.com/Ejexi/Agent/internal/tools/implementations/echo"
	"github.com/Ejexi/Agent/internal/tools/registry"
)

func main() {
	fmt.Println(" Testing Core Components...\n")

	// ========== Test 1: Memory ==========
	fmt.Println(" Test 1: Memory")
	mem := memory.New(5) // Keep max 5 messages

	// Add some messages
	mem.AddMessage(memory.Message{Role: "user", Content: "Hello!"})
	mem.AddMessage(memory.Message{Role: "assistant", Content: "Hi there!"})
	mem.AddMessage(memory.Message{Role: "user", Content: "How are you?"})

	// Get history return a version of the messages
	history := mem.GetHistory()
	fmt.Printf("   Messages in memory: %d\n", mem.Count()) //return number of messages
	for i, msg := range history {
		fmt.Printf("   [%d] %s: %s\n", i+1, msg.Role, msg.Content)
	}
	fmt.Println("  ## Memory works! ##\n")

	// ========== Test 2: Tool Registry ==========
	fmt.Println(" Test 2: Tool Registry")
	reg := registry.New() //return *Registry

	// Register echo tool
	echoTool := echo.New()
	err := reg.Register(echoTool)
	if err != nil {
		panic(err)
	}

	fmt.Printf("   Registered tools: %d\n", reg.Count())
	fmt.Printf("   Tool name: %s\n", echoTool.Name())
	fmt.Printf("   Tool description: %s\n", echoTool.Description())

	// Get tool from registry
	tool, err := reg.Get("echo")
	if err != nil {
		panic(err)
	}
	fmt.Printf("   Retrieved tool: %s\n", tool.Name())
	fmt.Println("  ## Registry works! ##\n")

	// ========== Test 3: Echo Tool Execution ==========
	fmt.Println(" Test 3: Echo Tool Execution")

	// Create parameters
	params := base.ToolParameters{
		InputData: map[string]interface{}{
			"message": "Hello from the tool system!",
		},
	}

	// Execute the tool
	ctx := context.Background()
	result, err := echoTool.Execute(ctx, params)
	if err != nil {
		panic(err)
	}

	// Check result
	if result.Success {
		fmt.Printf("  ## Tool executed successfully! ##\n")
		fmt.Printf("   Result: %v\n", result.Data)
		fmt.Printf("   Duration: %v\n", result.Duration)
	} else {
		fmt.Printf("   ❌ Tool failed: %v\n", result.Error)
	}
	fmt.Println()

	// ========== Test 4: Tool with Invalid Input ==========
	fmt.Println("❌ Test 4: Echo Tool with Invalid Input")

	// Try with missing parameter
	invalidParams := base.ToolParameters{
		InputData: map[string]interface{}{
			// No "message" parameter
		},
	}

	result, err = echoTool.Execute(ctx, invalidParams)
	if err != nil {
		fmt.Printf("  ## Correctly caught error: %v ##\n", err)
	} else {
		fmt.Printf("   ❌ Should have failed but didn't!\n")
	}
	fmt.Println()

	// ========== Test 5: List All Tools ==========
	fmt.Println(" Test 5: List All Tools")
	allTools := reg.List()
	fmt.Printf("   Total tools: %d\n", len(allTools))
	for i, t := range allTools {
		fmt.Printf("   [%d] %s - %s\n", i+1, t.Name(), t.Description())
	}
	fmt.Println(" ## Listing works! ## \n")

	fmt.Println("==================================================")
	fmt.Println(" ^~^ All tests passed!")
	fmt.Println("==================================================")
}
