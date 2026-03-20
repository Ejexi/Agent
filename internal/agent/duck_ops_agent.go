package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	shared_ports "github.com/SecDuckOps/shared/ports"
)

// DuckOpsAgent is the conversational CLI agent.
// It owns the REPL loop and bridges natural language to MasterAgent.
type DuckOpsAgent struct {
	master        *MasterAgent
	report        *ReportAgent
	logger        shared_ports.Logger
	cwd           string
	defaultFormat string // "text" | "json" | "sarif" — set by --format flag
}

// NewDuckOpsAgent creates a ready-to-run conversational agent.
func NewDuckOpsAgent(master *MasterAgent, report *ReportAgent, logger shared_ports.Logger) *DuckOpsAgent {
	cwd, _ := os.Getwd()
	return &DuckOpsAgent{
		master: master,
		report: report,
		logger: logger,
		cwd:    cwd,
	}
}

// WithFormat sets the default output format (used by --format flag).
func (a *DuckOpsAgent) WithFormat(format string) *DuckOpsAgent {
	a.defaultFormat = format
	return a
}

// Run starts the conversational REPL. Blocks until user exits.
func (a *DuckOpsAgent) Run(ctx context.Context) {
	fmt.Print(Render("# 🦆 DuckOps\n\n**Ready.** What would you like to do? *(type `exit` to quit)*\n"))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Print(Render("*Bye!* 👋\n"))
			break
		}

		a.handleInput(ctx, input)
		fmt.Println()
	}
}

// handleInput parses intent and dispatches to the right handler.
func (a *DuckOpsAgent) handleInput(ctx context.Context, input string) {
	md, err := a.HandleNaturalLanguage(ctx, input)
	if err != nil {
		fmt.Print(Render(fmt.Sprintf("## ❌ Error\n\n```\n%v\n```\n", err)))
		return
	}
	fmt.Print(Render(md))
}

// HandleNaturalLanguage is the NL → action bridge.
// Returns Markdown string. Public so HTTP handlers can call it too.
func (a *DuckOpsAgent) HandleNaturalLanguage(ctx context.Context, input string) (string, error) {
	intent := ParseIntent(input)
	target := resolvePath(intent.Target, a.cwd)

	switch intent.Action {
	case "scan":
		req := intentToScanRequest(intent, target)
		result, err := a.master.HandleScanRequest(ctx, req)
		if err != nil {
			return "", err
		}
		// CLI --format flag takes precedence over intent-parsed format
		outputFmt := intent.OutputFmt
		if outputFmt == "" {
			outputFmt = a.defaultFormat
		}
		switch outputFmt {
		case "json":
			b, _ := json.MarshalIndent(result, "", "  ")
			return fmt.Sprintf("```json\n%s\n```\n", string(b)), nil
		case "sarif":
			if err := a.report.Process(ctx, result, "sarif"); err != nil {
				return "", err
			}
			return "", nil // SARIF written directly to stdout
		default:
			return FormatMarkdown(result), nil
		}

	case "status":
		return StatusMarkdown(a.getStatus(ctx)), nil

	case "prefetch":
		return StatusMarkdown(a.prefetchImages(ctx)), nil

	case "help":
		return HelpMarkdown(), nil

	default:
		req := ScanRequest{TargetPath: target}
		result, err := a.master.HandleScanRequest(ctx, req)
		if err != nil {
			return "", err
		}
		return FormatMarkdown(result), nil
	}
}

// getStatus returns scanner service health.
func (a *DuckOpsAgent) getStatus(ctx context.Context) string {
	if a.master == nil || a.master.scannerSvc == nil {
		return "Scanner service not configured. Add [[mcp.servers]] with name='scanner' to ~/.duckops/config.toml."
	}
	available := a.master.scannerSvc.AvailableScanners()
	return fmt.Sprintf("Scanner service ready via MCP. Available scanners: %v", available)
}

// prefetchImages is kept for CLI compatibility but is a no-op with MCP backend.
// MCP scanner servers manage their own tool availability.
func (a *DuckOpsAgent) prefetchImages(ctx context.Context) string {
	if a.master == nil || a.master.scannerSvc == nil {
		return "Scanner service not configured — nothing to prefetch."
	}
	available := a.master.scannerSvc.AvailableScanners()
	return fmt.Sprintf("MCP scanner backend active. %d scanners available: %v", len(available), available)
}

// WarmupAll is a no-op with the MCP backend — servers manage their own readiness.
func (a *DuckOpsAgent) WarmupAll(ctx context.Context) {
	// MCP servers warm up on connection, not on demand.
}

// intentToScanRequest converts a parsed Intent to a structured ScanRequest.
func intentToScanRequest(intent Intent, resolvedTarget string) ScanRequest {
	req := ScanRequest{
		TargetPath:  resolvedTarget,
		Categories:  intent.Categories,
		MinSeverity: intent.Severity,
		OutputFmt:   intent.OutputFmt,
	}

	// If no specific categories, enable all
	if len(intent.Categories) == 0 {
		req.RunSAST = true
		req.RunSCA = true
		req.RunSecrets = true
		req.RunIaC = true
		req.RunDeps = true
	}

	return req
}
