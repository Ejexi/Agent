package agent

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
)

// renderer is a package-level TermRenderer, initialized once.
// Uses WithAutoStyle() so it picks dark/light based on terminal background.
var renderer *glamour.TermRenderer

func init() {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		// fallback: renderer stays nil, Render() will return plain text
		return
	}
	renderer = r
}

// Render takes a markdown string and returns glamour-styled ANSI output.
// Falls back to plain text if glamour is unavailable.
func Render(md string) string {
	if renderer == nil {
		return md
	}
	out, err := renderer.Render(md)
	if err != nil {
		return md
	}
	return out
}

// ── Markdown builders ────────────────────────────────────────────────────────
// These replace FormatText — output is now Markdown that glamour renders.

// FormatMarkdown converts an AggregatedResult to a Markdown document.
func FormatMarkdown(result *AggregatedResult) string {
	var sb strings.Builder

	// ── Header ───────────────────────────────────────────────────────────────
	sb.WriteString(fmt.Sprintf("# 🦆 DuckOps Scan Report\n\n"))
	sb.WriteString(fmt.Sprintf("**Target:** `%s`\n\n", result.TargetPath))

	// ── Orchestrator intelligence ─────────────────────────────────────────────
	if result.Plan != nil && result.Plan.SystemSummary != "" {
		sb.WriteString("## 🧠 Orchestrator Analysis\n\n")
		sb.WriteString(fmt.Sprintf("%s\n\n", result.Plan.SystemSummary))

		if result.Plan.RiskOverview != "" {
			sb.WriteString(fmt.Sprintf("> **Risk:** %s\n\n", result.Plan.RiskOverview))
		}

		if len(result.Plan.DetectedSignals) > 0 {
			sb.WriteString("### Detected Signals\n\n")
			for _, s := range result.Plan.DetectedSignals {
				if s.Weight >= 0.5 {
					bar := weightBar(s.Weight)
					sb.WriteString(fmt.Sprintf("- `%s` %s — %s\n", s.Type, bar, s.Reason))
				}
			}
			sb.WriteString("\n")
		}

		if result.Plan.Confidence > 0 {
			sb.WriteString(fmt.Sprintf("**Confidence:** %.0f%%\n\n", result.Plan.Confidence*100))
		}
	}

	// ── Scan summary ─────────────────────────────────────────────────────────
	sb.WriteString("## 📊 Summary\n\n")
	sb.WriteString("| Severity | Count |\n")
	sb.WriteString("|----------|-------|\n")
	if result.Summary.Critical > 0 {
		sb.WriteString(fmt.Sprintf("| 🔴 Critical | **%d** |\n", result.Summary.Critical))
	}
	if result.Summary.High > 0 {
		sb.WriteString(fmt.Sprintf("| 🟠 High | **%d** |\n", result.Summary.High))
	}
	if result.Summary.Medium > 0 {
		sb.WriteString(fmt.Sprintf("| 🟡 Medium | %d |\n", result.Summary.Medium))
	}
	if result.Summary.Low > 0 {
		sb.WriteString(fmt.Sprintf("| 🟢 Low | %d |\n", result.Summary.Low))
	}
	if result.Summary.Info > 0 {
		sb.WriteString(fmt.Sprintf("| ℹ️  Info | %d |\n", result.Summary.Info))
	}
	sb.WriteString(fmt.Sprintf("| **Total** | **%d** |\n\n", result.Summary.Total))

	// ── Per-category status ───────────────────────────────────────────────────
	sb.WriteString("## 🔍 Scan Categories\n\n")
	for _, task := range result.Tasks {
		icon := "✅"
		detail := fmt.Sprintf("%d findings", len(task.Findings))
		switch task.Status {
		case ScanTaskFailed:
			icon = "❌"
			detail = task.Error
		case ScanTaskTimedOut:
			icon = "⏱️"
			detail = "timed out"
		}
		sb.WriteString(fmt.Sprintf("- %s **%s** — %s\n", icon, strings.ToUpper(string(task.Category)), detail))
		if task.OrchestratorInstruction != "" {
			// Show first line of instruction as context
			lines := strings.SplitN(task.OrchestratorInstruction, "\n", 2)
			sb.WriteString(fmt.Sprintf("  *%s*\n", lines[0]))
		}
	}
	sb.WriteString("\n")

	if result.Summary.Total == 0 {
		sb.WriteString("## ✅ No Findings\n\nNo security issues detected.\n")
		return sb.String()
	}

	// ── Findings by severity ──────────────────────────────────────────────────
	sb.WriteString("## 🚨 Findings\n\n")

	type sevGroup struct {
		icon  string
		label string
		sev   string
	}
	groups := []sevGroup{
		{"🔴", "Critical", "CRITICAL"},
		{"🟠", "High", "HIGH"},
		{"🟡", "Medium", "MEDIUM"},
		{"🟢", "Low", "LOW"},
		{"ℹ️", "Info", "INFO"},
	}

	for _, g := range groups {
		var matching []string
		for _, f := range result.AllFindings {
			if string(f.Severity) != g.sev {
				continue
			}
			var loc string
			if f.Location.File != "" {
				loc = fmt.Sprintf("`%s`", f.Location.File)
				if f.Location.Line > 0 {
					loc = fmt.Sprintf("`%s:%d`", f.Location.File, f.Location.Line)
				}
			} else if f.File != "" {
				loc = fmt.Sprintf("`%s`", f.File)
			}

			entry := fmt.Sprintf("#### %s %s\n\n", g.icon, f.Title)
			if f.CVE != "" {
				entry += fmt.Sprintf("**CVE:** `%s`  \n", f.CVE)
			}
			if f.Scanner != "" {
				entry += fmt.Sprintf("**Scanner:** %s  \n", f.Scanner)
			}
			if loc != "" {
				entry += fmt.Sprintf("**Location:** %s  \n", loc)
			}
			if f.Description != "" {
				entry += fmt.Sprintf("\n%s\n", f.Description)
			}
			if f.Remediation != "" {
				entry += fmt.Sprintf("\n> **Fix:** %s\n", f.Remediation)
			}
			entry += "\n"
			matching = append(matching, entry)
		}

		if len(matching) > 0 {
			sb.WriteString(fmt.Sprintf("### %s %s (%d)\n\n", g.icon, g.label, len(matching)))
			for _, m := range matching {
				sb.WriteString(m)
			}
		}
	}

	return sb.String()
}

// weightBar converts a 0.0-1.0 weight to a simple text bar.
func weightBar(w float64) string {
	filled := int(w * 5)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 5-filled)
	return fmt.Sprintf("[%s %.0f%%]", bar, w*100)
}

// StatusMarkdown formats a simple status message as Markdown.
func StatusMarkdown(msg string) string {
	return fmt.Sprintf("## 🦆 DuckOps Status\n\n%s\n", msg)
}

// HelpMarkdown returns the help text as Markdown.
func HelpMarkdown() string {
	return `# 🦆 DuckOps — DevSecOps AI Agent

Just tell me what you want:

| Example | What it does |
|---------|-------------|
| ` + "`scan this project`" + ` | Full scan on current directory |
| ` + "`check for secrets only`" + ` | Run secrets scanners only |
| ` + "`scan ./src for critical issues`" + ` | Target path + severity filter |
| ` + "`look for vulnerabilities in ./backend`" + ` | SCA + SAST on a subdirectory |
| ` + "`is docker ready?`" + ` | Check Docker + image cache status |
| ` + "`download scanner images`" + ` | Pre-warm all scanner images |

I'll figure out the rest.
`
}
