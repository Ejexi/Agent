package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCommandDiscovery(t *testing.T) {
	// Create a temporary directory for our test "PATH"
	tmpDir, err := os.MkdirTemp("", "duckops-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy executable
	testCmd := "duck-test-cmd"
	if runtime.GOOS == "windows" {
		testCmd += ".exe"
	}
	cmdPath := filepath.Join(tmpDir, testCmd)
	if err := os.WriteFile(cmdPath, []byte("echo hello"), 0755); err != nil {
		t.Fatalf("failed to create test cmd: %v", err)
	}

	// Override PATH for discovery
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir)
	defer os.Setenv("PATH", oldPath)

	discovery := NewCommandDiscovery()
	// Discovery runs in background, so we give it a moment
	// or call Refresh synchronously for the test
	discovery.Refresh()

	results := discovery.Search("duck")
	if len(results) == 0 {
		t.Errorf("expected to find test command, got 0 results")
	}

	found := false
	for _, r := range results {
		if r.Name == testCmd {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("test command %q not found in results: %+v", testCmd, results)
	}
}
