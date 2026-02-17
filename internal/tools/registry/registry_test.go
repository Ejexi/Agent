package registry

import (
	"testing"

	"github.com/Ejexi/Agent/internal/tools/implementations/echo"
)

func TestRegistry(t *testing.T) {
	reg := New()

	// Register a tool
	tool := echo.New()
	err := reg.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Check Count
	if reg.Count() != 1 {
		t.Errorf("Expected 1 tool, got %d", reg.Count())
	}
	// Get the tool
	retrieved, err := reg.Get("echo")
	if err != nil {
		t.Fatalf("Failed to get tool: %v", err)
	}
	if retrieved.Name() != "echo" {
		t.Errorf("Expected tool name 'echo', got %s", retrieved.Name())
	}
}

func TestRegistryDuplicate(t *testing.T) {
	reg := New()
	tool := echo.New()

	// Register once
	reg.Register(tool)

	// Try to register again (should fail)
	err := reg.Register(tool)
	if err == nil {
		t.Error("Should have failed when registering duplicate tool")
	}
}
