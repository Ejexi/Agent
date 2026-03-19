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
	master *MasterAgent
	report *ReportAgent
	logger shared_ports.Logger
	cwd    string
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
		if intent.OutputFmt == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			return fmt.Sprintf("```json\n%s\n```\n", string(b)), nil
		}
		return FormatMarkdown(result), nil

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

// getStatus returns Docker health + cached image summary.
func (a *DuckOpsAgent) getStatus(ctx context.Context) string {
	if a.master == nil || a.master.scannerSvc == nil {
		return "Docker is not available. Start Docker and run duckops again."
	}
	return "Docker is running. Scanner service ready.\n" +
		"Tip: run \"download scanner images\" to pre-warm all scanner images."
}

// prefetchImages triggers WarmupImages in the background and reports progress.
func (a *DuckOpsAgent) prefetchImages(ctx context.Context) string {
	if a.master == nil || a.master.scannerSvc == nil {
		return "Docker is not available — cannot prefetch images."
	}

	// Collect all scanner names from the subagentScanners map
	var all []string
	seen := make(map[string]bool)
	for _, scanners := range subagentScanners {
		for _, s := range scanners {
			if !seen[s] {
				all = append(all, s)
				seen[s] = true
			}
		}
	}

	fmt.Printf("Downloading %d scanner images in the background...\n", len(all))
	go func() {
		// DockerWarden.WarmupImages handles this via ScannerPort
		// We reach it through the aggregator's underlying warden
		if wp, ok := interface{}(a.master.scannerSvc).(interface {
			WarmupImages(ctx context.Context, names []string) error
		}); ok {
			_ = wp.WarmupImages(ctx, all)
		}
	}()

	return fmt.Sprintf("Pulling %d images in the background. Run 'is docker ready?' to check status.", len(all))
}

// WarmupAll pulls all scanner images in the background at agent startup.
// Called once from root.go — non-blocking.
func (a *DuckOpsAgent) WarmupAll(ctx context.Context) {
	if a.master == nil || a.master.scannerSvc == nil {
		return
	}
	var all []string
	seen := make(map[string]bool)
	for _, scanners := range subagentScanners {
		for _, s := range scanners {
			if !seen[s] {
				all = append(all, s)
				seen[s] = true
			}
		}
	}
	if wp, ok := interface{}(a.master.scannerSvc).(interface {
		WarmupImages(ctx context.Context, names []string) error
	}); ok {
		_ = wp.WarmupImages(ctx, all)
	}
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
