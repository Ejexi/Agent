package subagents

import (
	"context"
	"fmt"
	"strings"

	llm_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/scanner/domain"
	scanner_ports "github.com/SecDuckOps/shared/scanner/ports"
)

// intelligentBase extends base with LLM-driven scanner selection and insight generation.
// Every intelligent subagent embeds this instead of base.
type intelligentBase struct {
	scannerSvc      scanner_ports.ScannerServicePort
	llm             llm_domain.LLM   // nil = fall back to running all scanners
	defaultScanners []string
	categoryName    string
	systemPrompt    string
}

// selectScanners asks the LLM which scanners to run given project signals.
// Falls back to all defaultScanners if LLM is unavailable or fails.
func (b *intelligentBase) selectScanners(ctx context.Context, signals ProjectSignals) []string {
	if b.llm == nil {
		return b.defaultScanners
	}

	prompt := b.buildSelectionPrompt(signals)

	var rec ScannerRecommendation
	err := b.llm.GenerateJSON(ctx, []llm_domain.Message{
		{Role: llm_domain.RoleSystem, Content: b.systemPrompt},
		{Role: llm_domain.RoleUser, Content: prompt},
	}, nil, &rec)

	if err != nil || len(rec.Scanners) == 0 {
		// LLM failed — fall back to all defaults (non-fatal)
		return b.defaultScanners
	}

	// Whitelist: only return scanners that are in our allowed list
	allowed := make(map[string]bool)
	for _, s := range b.defaultScanners {
		allowed[s] = true
	}
	var filtered []string
	for _, s := range rec.Scanners {
		if allowed[s] {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) == 0 {
		return b.defaultScanners
	}
	return filtered
}

// buildSelectionPrompt creates the scanner selection prompt.
func (b *intelligentBase) buildSelectionPrompt(signals ProjectSignals) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are selecting security scanners for a %s scan.\n\n", b.categoryName))
	sb.WriteString("## Project signals detected\n")
	sb.WriteString(fmt.Sprintf("- Languages: %s\n", strings.Join(signals.Languages, ", ")))
	sb.WriteString(fmt.Sprintf("- Frameworks: %s\n", strings.Join(signals.Frameworks, ", ")))
	sb.WriteString(fmt.Sprintf("- Has IaC: %v\n", signals.HasIaC))
	sb.WriteString(fmt.Sprintf("- Has Docker: %v\n", signals.HasDocker))
	sb.WriteString(fmt.Sprintf("- Has Tests: %v\n", signals.HasTests))
	sb.WriteString(fmt.Sprintf("- File count: %d\n", signals.FileCount))
	if len(signals.RootFiles) > 0 {
		sb.WriteString(fmt.Sprintf("- Root files: %s\n", strings.Join(signals.RootFiles[:min(20, len(signals.RootFiles))], ", ")))
	}

	sb.WriteString(fmt.Sprintf("\n## Available scanners for %s\n", b.categoryName))
	for _, s := range b.defaultScanners {
		sb.WriteString(fmt.Sprintf("- %s\n", s))
	}

	sb.WriteString(`
## Your task
Select ONLY the scanners that are relevant for this specific project.
Skip scanners designed for languages/frameworks not present.
Order by priority (most relevant first).

Respond with ONLY valid JSON in this exact format:
{
  "scanners": ["scanner1", "scanner2"],
  "rationale": "brief explanation",
  "skip_reason": "why others were skipped (optional)"
}`)

	return sb.String()
}

// interpretFindings asks the LLM to add strategic context to raw findings.
func (b *intelligentBase) interpretFindings(ctx context.Context, findings []domain.Finding, signals ProjectSignals) string {
	if b.llm == nil || len(findings) == 0 {
		return ""
	}

	// Build a compact findings summary (avoid sending too many tokens)
	summary := buildFindingsSummary(findings)

	result, err := b.llm.Generate(ctx, []llm_domain.Message{
		{Role: llm_domain.RoleSystem, Content: b.systemPrompt},
		{
			Role: llm_domain.RoleUser,
			Content: fmt.Sprintf(`## %s Scan Results

Project signals: languages=%s, frameworks=%s, has_iac=%v

Findings summary:
%s

## Your task
As a Staff+ security engineer, provide:
1. The top 2-3 most critical risks that need immediate attention
2. Any architectural patterns that increase risk exposure
3. One concrete remediation priority

Keep response under 150 words. Be specific, not generic.`,
				strings.ToUpper(b.categoryName),
				strings.Join(signals.Languages, ","),
				strings.Join(signals.Frameworks, ","),
				signals.HasIaC,
				summary,
			),
		},
	}, nil)

	if err != nil {
		return ""
	}
	return strings.TrimSpace(result.Content)
}

// runIntelligent is the full intelligent execution flow:
// 1. Analyze project
// 2. LLM selects relevant scanners
// 3. Run selected scanners
// 4. LLM interprets findings
func (b *intelligentBase) runIntelligent(ctx context.Context, targetPath string) ([]domain.Finding, string, error) {
	// Phase 1: fast project signal collection (no LLM)
	signals := AnalyzeProject(targetPath)

	// Phase 2: LLM selects scanners (falls back to all if LLM unavailable)
	selected := b.selectScanners(ctx, signals)

	// Phase 3: run selected scanners
	var all []domain.Finding
	for _, name := range selected {
		result, err := b.scannerSvc.RunScan(ctx, targetPath, name)
		if err != nil {
			continue // non-fatal
		}
		all = append(all, result.Findings...)
	}

	// Phase 4: LLM interprets findings (optional enrichment)
	insight := b.interpretFindings(ctx, all, signals)

	return all, insight, nil
}

// buildFindingsSummary creates a compact text summary of findings for the LLM.
func buildFindingsSummary(findings []domain.Finding) string {
	counts := make(map[domain.Severity]int)
	for _, f := range findings {
		counts[f.Severity]++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total: %d findings\n", len(findings)))
	for _, sev := range []domain.Severity{
		domain.SeverityCritical, domain.SeverityHigh,
		domain.SeverityMedium, domain.SeverityLow,
	} {
		if n := counts[sev]; n > 0 {
			sb.WriteString(fmt.Sprintf("- %s: %d\n", sev, n))
		}
	}

	// Include top 5 critical/high findings for context
	shown := 0
	sb.WriteString("\nTop findings:\n")
	for _, f := range findings {
		if shown >= 5 {
			break
		}
		if f.Severity == domain.SeverityCritical || f.Severity == domain.SeverityHigh {
			sb.WriteString(fmt.Sprintf("- [%s] %s (scanner: %s)\n", f.Severity, f.Title, f.Scanner))
			shown++
		}
	}
	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
