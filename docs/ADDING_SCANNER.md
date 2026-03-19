# Adding a New Scanner to DuckOps

This guide walks through adding a new security scanner end-to-end.

---

## 1. Write the Parser (`shared/scanner/parsers/`)

Choose the right category directory:

```
shared/scanner/parsers/
├── sast/           (semgrep, gosec, bandit, njsscan, brakeman)
├── container/      (trivy, grype)
├── secrets/        (gitleaks, trufflehog, detectsecrets)
├── iac/            (checkov, tfsec, kics, terrascan, tflint)
├── dependency/     (osvscanner, dependencycheck)
└── dast/           (zap, nuclei)
```

Create `shared/scanner/parsers/<category>/<scanner>/<scanner>.go`:

```go
package myparser

import (
    "encoding/json"
    "github.com/SecDuckOps/shared/scanner/domain"
)

type MyParser struct{}

// ScannerName returns the scanner identifier used in ScanOpts.
func (p *MyParser) ScannerName() string { return "myscanner" }

// SupportedFormats declares the output formats this parser handles.
func (p *MyParser) SupportedFormats() []string { return []string{"json"} }

// GetScanCommand returns the CLI args to run the scanner inside the container.
// workspacePath is always /scan/workspace inside the container.
func (p *MyParser) GetScanCommand(workspacePath string) []string {
    return []string{
        "myscanner",
        "--format", "json",
        "--output", "/dev/stdout",
        workspacePath,
    }
}

// Parse converts raw scanner stdout into DuckOps findings.
func (p *MyParser) Parse(output []byte) ([]domain.Finding, error) {
    var report MyReport
    if err := json.Unmarshal(output, &report); err != nil {
        return nil, err
    }

    var findings []domain.Finding
    for _, issue := range report.Issues {
        findings = append(findings, domain.Finding{
            ID:          issue.ID,
            Title:       issue.Title,
            Description: issue.Description,
            Severity:    mapSeverity(issue.Severity),
            Scanner:     p.ScannerName(),
            Location: domain.Location{
                File:   issue.Filename,
                Line:   issue.Line,
                Column: issue.Column,
            },
            Remediation: issue.Fix,
            CVE:         issue.CVE,
            Metadata: map[string]any{
                "rule_id": issue.RuleID,
                "tags":    issue.Tags,
            },
        })
    }
    return findings, nil
}

func mapSeverity(s string) domain.Severity {
    switch s {
    case "CRITICAL", "critical":
        return domain.SeverityCritical
    case "HIGH", "high", "error":
        return domain.SeverityHigh
    case "MEDIUM", "medium", "warning":
        return domain.SeverityMedium
    case "LOW", "low", "info":
        return domain.SeverityLow
    default:
        return domain.SeverityInfo
    }
}

// MyReport mirrors the scanner's JSON output structure.
type MyReport struct {
    Issues []MyIssue `json:"issues"`
}

type MyIssue struct {
    ID          string `json:"id"`
    RuleID      string `json:"rule_id"`
    Title       string `json:"title"`
    Description string `json:"description"`
    Severity    string `json:"severity"`
    Filename    string `json:"filename"`
    Line        int    `json:"line"`
    Column      int    `json:"column"`
    Fix         string `json:"fix"`
    CVE         string `json:"cve"`
    Tags        []string `json:"tags"`
}
```

---

## 2. Register the Docker Image (`agent/internal/adapters/warden/image_registry.go`)

```go
var DefaultImageRegistry = map[string]string{
    // ... existing entries ...
    "myscanner": "vendor/myscanner:latest",  // add this
}
```

---

## 3. Register the Parser (`agent/internal/adapters/bootstrap/bootstrap.go`)

In the `registerTools` function, add your parser to the parsers slice:

```go
import myparser "github.com/SecDuckOps/shared/scanner/parsers/sast/myparser"

// Inside registerTools:
parsers := []ports.ResultParserPort{
    // ... existing parsers ...
    &myparser.MyParser{},  // add this
}
```

---

## 4. Add to Subagent Allowed List (`agent/internal/agent/types.go`)

```go
var subagentScanners = map[ScanCategory][]string{
    CategorySAST: {"semgrep", "gosec", "bandit", "njsscan", "brakeman", "myscanner"}, // add
    // ...
}
```

---

## 5. Update Subagent System Prompt (`agent/internal/agent/subagents/subagents.go`)

Update the relevant subagent's `systemPrompt` constant to describe when your scanner should be selected:

```go
const sastSystemPrompt = `...
Scanner guidance:
- semgrep: universal, good for all languages — always include
- gosec: Go-specific — only if Go is detected
- myscanner: Rust-specific — only if Rust is detected   ← add this
...`
```

---

## 6. Write a Parser Test

```go
package myparser_test

import (
    "os"
    "testing"
    myparser "github.com/SecDuckOps/shared/scanner/parsers/sast/myparser"
)

func TestParse_ValidJSON(t *testing.T) {
    p := &myparser.MyParser{}
    data, _ := os.ReadFile("testdata/sample_output.json")
    findings, err := p.Parse(data)
    if err != nil {
        t.Fatal(err)
    }
    if len(findings) == 0 {
        t.Error("expected at least one finding")
    }
}

func TestParse_EmptyOutput(t *testing.T) {
    p := &myparser.MyParser{}
    findings, err := p.Parse([]byte(`{"issues": []}`))
    if err != nil {
        t.Fatal(err)
    }
    if len(findings) != 0 {
        t.Errorf("expected 0 findings, got %d", len(findings))
    }
}

func TestParse_InvalidJSON(t *testing.T) {
    p := &myparser.MyParser{}
    _, err := p.Parse([]byte("not json"))
    if err == nil {
        t.Error("expected error for invalid JSON")
    }
}
```

Add `testdata/sample_output.json` with real scanner output.

---

## Checklist

```
[ ] Parser implements ResultParserPort (ScannerName, SupportedFormats, GetScanCommand, Parse)
[ ] GetScanCommand uses /scan/workspace — not a hardcoded path
[ ] mapSeverity handles all possible values from the scanner
[ ] Docker image added to image_registry.go
[ ] Parser registered in bootstrap.go parsers slice
[ ] Scanner name added to subagentScanners in types.go
[ ] Subagent system prompt updated with guidance
[ ] Parser unit tests added (valid, empty, invalid JSON)
[ ] testdata/ sample output file committed
```
