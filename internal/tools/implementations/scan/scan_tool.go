package scan

import (
	"context"

	"github.com/SecDuckOps/shared/scanner/aggregator"
	"github.com/SecDuckOps/shared/types"

	agent_domain "github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/tools/base"
)

// ScanParams defines the typed parameters for the scan tool.
type ScanParams struct {
	Target  string `json:"target"`
	Scanner string `json:"scanner"`
}

// ScanTool performs security scans via DockerWarden and specific parsers.
type ScanTool struct {
	base.BaseTypedTool[ScanParams]
	scannerSvc *aggregator.ScannerService
}

// NewScanTool creates a new ScanTool.
func NewScanTool(scannerSvc *aggregator.ScannerService) *ScanTool {
	t := &ScanTool{
		scannerSvc: scannerSvc,
	}
	t.Impl = t
	return t
}

func (t *ScanTool) Name() string { return "scan" }

func (t *ScanTool) Schema() agent_domain.ToolSchema {
	return agent_domain.ToolSchema{
		Name:        "scan",
		Description: "Perform a security scan on a target using a specific scanner engine.",
		Parameters: map[string]string{
			"target":  "string - The directory or file to scan. IMPORTANT: Use '.' to scan the current project workspace. DO NOT use absolute Linux paths like '/vuln' or '/app' as they will fail on Windows hosts.",
			"scanner": "string - The scanner engine to use (e.g. 'trivy', 'semgrep', 'gitleaks', 'zap', 'tfsec', 'gosec', etc.)",
		},
	}
}

func (t *ScanTool) ParseParams(input map[string]interface{}) (ScanParams, error) {
	params, err := base.DefaultParseParams[ScanParams](input)
	if err != nil {
		return params, err
	}
	if params.Target == "" {
		return params, types.New(types.ErrCodeInvalidInput, "missing 'target' argument")
	}
	
	// Auto-correct common LLM DevSecOps target hallucinations
	if params.Target == "/vuln" || params.Target == "/app" || params.Target == "/src" {
		params.Target = "."
	}

	if params.Scanner == "" {
		return params, types.New(types.ErrCodeInvalidInput, "missing 'scanner' argument")
	}
	return params, nil
}

func (t *ScanTool) Execute(ctx context.Context, params ScanParams) (agent_domain.Result, error) {
	if t.scannerSvc == nil {
		return agent_domain.Result{
			Success: false,
			Status:  "Docker Warden is not available",
			Data: map[string]interface{}{
				"error": "The scanner service is not initialized. Please ensure Docker is running and healthy.",
			},
		}, nil
	}

	scanResult, err := t.scannerSvc.RunScan(ctx, params.Target, params.Scanner)
	if err != nil {
		return agent_domain.Result{
			Success: false,
			Status:  "scan execution failed",
			Data: map[string]interface{}{
				"error": err.Error(),
			},
		}, nil
	}

	status := "completed"
	exitCode := 0
	if scanResult.Error != "" {
		status = "failed"
		exitCode = 1 // or another non-zero value to represent error
	}

	return agent_domain.Result{
		Success: true,
		Status:  "scan completed dynamically",
		Data: map[string]interface{}{
			"status":         status,
			"findings_count": len(scanResult.Findings),
			"error":          scanResult.Error,
			"target_passed":  params.Target,
			"duration_ms":    scanResult.EndTime.Sub(scanResult.StartTime).Milliseconds(),
			"exit_code":      exitCode,
			"SYSTEM_NOTE":    "Scan is fully complete. Do not re-run this scan. Formulate your final response evaluating the count and any errors.",
		},
	}, nil
}
