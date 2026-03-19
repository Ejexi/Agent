package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/SecDuckOps/agent/internal/domain/mcp"
	scanner_domain "github.com/SecDuckOps/shared/scanner/domain"
	scanner_ports "github.com/SecDuckOps/shared/scanner/ports"
	"github.com/SecDuckOps/shared/types"
)

// ScannerAdapter implements scanner_ports.ScannerServicePort by routing
// scan calls through an MCP server that hosts scanner tools.
//
// Expected MCP server tool naming convention:
//   scanner__<name>   e.g.  scanner__trivy, scanner__semgrep, scanner__gitleaks
//
// The MCP server tool receives:
//   { "target": "/path/to/scan" }
//
// And returns JSON matching scanner_domain.ScanResult.
type ScannerAdapter struct {
	client     *Client
	serverName string // which MCP server holds the scanner tools
	mu         sync.RWMutex
	toolCache  []string // cached list of available scanner names
}

// NewScannerAdapter creates a ScannerAdapter pointing at the named MCP server.
// serverName must match a ServerConfig.Name in the MCP config.
func NewScannerAdapter(client *Client, serverName string) *ScannerAdapter {
	return &ScannerAdapter{
		client:     client,
		serverName: serverName,
	}
}

// RunScan calls the named scanner via MCP and returns parsed findings.
func (a *ScannerAdapter) RunScan(ctx context.Context, target string, scannerName string) (scanner_domain.ScanResult, error) {
	toolName := fmt.Sprintf("scanner__%s", scannerName)

	result, err := a.client.CallTool(ctx, mcp.ToolCall{
		ServerName: a.serverName,
		ToolName:   toolName,
		Arguments: map[string]interface{}{
			"target": target,
		},
	})
	if err != nil {
		return scanner_domain.ScanResult{}, types.Wrapf(err, types.ErrCodeExecutionFailed,
			"mcp scanner %q failed", scannerName)
	}
	if result.IsError {
		return scanner_domain.ScanResult{
			Error: result.Content,
		}, nil
	}

	var scanResult scanner_domain.ScanResult
	if err := json.Unmarshal([]byte(result.Content), &scanResult); err != nil {
		// Non-JSON response — return as raw output
		return scanner_domain.ScanResult{
			RawOutput: result.Content,
		}, nil
	}
	return scanResult, nil
}

// RunScanBatch executes multiple scanners in parallel via MCP.
func (a *ScannerAdapter) RunScanBatch(ctx context.Context, target string, scannerNames []string) []scanner_ports.ScanBatchResult {
	results := make([]scanner_ports.ScanBatchResult, len(scannerNames))
	var wg sync.WaitGroup
	for i, name := range scannerNames {
		wg.Add(1)
		go func(idx int, scannerName string) {
			defer wg.Done()
			result, err := a.RunScan(ctx, target, scannerName)
			results[idx] = scanner_ports.ScanBatchResult{
				ScannerName: scannerName,
				Result:      result,
				Err:         err,
			}
		}(i, name)
	}
	wg.Wait()
	return results
}

// HasScanner returns true if the scanner tool is available on the MCP server.
func (a *ScannerAdapter) HasScanner(scannerName string) bool {
	tools, _ := a.client.ListServerTools(context.Background(), a.serverName)
	want := fmt.Sprintf("scanner__%s", scannerName)
	for _, t := range tools {
		if t.Name == want {
			return true
		}
	}
	return false
}

// AvailableScanners returns scanner names by stripping the "scanner__" prefix.
func (a *ScannerAdapter) AvailableScanners() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.toolCache) > 0 {
		return a.toolCache
	}

	tools, _ := a.client.ListServerTools(context.Background(), a.serverName)
	var names []string
	for _, t := range tools {
		if len(t.Name) > 9 && t.Name[:9] == "scanner__" {
			names = append(names, t.Name[9:])
		}
	}
	a.toolCache = names
	return names
}
