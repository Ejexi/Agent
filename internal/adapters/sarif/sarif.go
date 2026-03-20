// Package sarif serialises DuckOps scan findings to SARIF 2.1.0.
//
// SARIF (Static Analysis Results Interchange Format) is an OASIS standard
// consumed natively by GitHub Advanced Security, VS Code, and most CI systems.
//
// Usage:
//
//	report := sarif.FromFindings("duckops", findings)
//	data, err := json.MarshalIndent(report, "", "  ")
//
// GitHub Actions integration:
//
//	- name: DuckOps security scan
//	  run: duckops --headless --format sarif . > results.sarif
//	- name: Upload SARIF
//	  uses: github/codeql-action/upload-sarif@v3
//	  with:
//	    sarif_file: results.sarif
package sarif

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/SecDuckOps/shared/scanner/domain"
)

const sarifVersion = "2.1.0"
const sarifSchema = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"

// Report is the top-level SARIF 2.1.0 document.
type Report struct {
	Version string `json:"version"`
	Schema  string `json:"$schema"`
	Runs    []Run  `json:"runs"`
}

// Run represents one invocation of one or more scanners.
type Run struct {
	Tool        Tool         `json:"tool"`
	Results     []Result     `json:"results"`
	Invocations []Invocation `json:"invocations,omitempty"`
}

// Tool describes the scanner that produced the results.
type Tool struct {
	Driver Driver `json:"driver"`
}

// Driver holds scanner metadata and the rule definitions.
type Driver struct {
	Name           string  `json:"name"`
	Version        string  `json:"version,omitempty"`
	InformationURI string  `json:"informationUri,omitempty"`
	Rules          []Rule  `json:"rules,omitempty"`
}

// Rule describes a single check / vulnerability class.
type Rule struct {
	ID               string          `json:"id"`
	Name             string          `json:"name,omitempty"`
	ShortDescription Message         `json:"shortDescription"`
	FullDescription  *Message        `json:"fullDescription,omitempty"`
	HelpURI          string          `json:"helpUri,omitempty"`
	Properties       *RuleProperties `json:"properties,omitempty"`
}

// RuleProperties carries severity tags understood by GitHub.
type RuleProperties struct {
	Tags            []string `json:"tags,omitempty"`
	SecuritySeverity string  `json:"security-severity,omitempty"` // CVSS-like 0–10 string
}

// Result is a single finding instance.
type Result struct {
	RuleID    string     `json:"ruleId"`
	Level     string     `json:"level"` // "error" | "warning" | "note"
	Message   Message    `json:"message"`
	Locations []Location `json:"locations,omitempty"`
	Fixes     []Fix      `json:"fixes,omitempty"`
}

// Message is a plain-text message container.
type Message struct {
	Text string `json:"text"`
}

// Location identifies the position of a finding in the source.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

// PhysicalLocation describes a file and optional region.
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           *Region          `json:"region,omitempty"`
}

// ArtifactLocation holds the file URI.
type ArtifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"`
}

// Region is a line/column range.
type Region struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

// Fix carries optional remediation text.
type Fix struct {
	Description Message `json:"description"`
}

// Invocation records execution metadata.
type Invocation struct {
	ExecutionSuccessful bool      `json:"executionSuccessful"`
	StartTimeUTC        time.Time `json:"startTimeUtc,omitempty"`
	EndTimeUTC          time.Time `json:"endTimeUtc,omitempty"`
}

// FromFindings converts a slice of DuckOps findings to a SARIF 2.1.0 Report.
// toolVersion is optional — pass "" to omit it.
func FromFindings(toolVersion string, findings []domain.Finding) Report {
	if len(findings) == 0 {
		return Report{
			Version: sarifVersion,
			Schema:  sarifSchema,
			Runs: []Run{{
				Tool:    duckOpsTool(toolVersion, nil),
				Results: []Result{},
			}},
		}
	}

	// Deduplicate rules — one Rule entry per unique finding ID/title combo.
	ruleIndex := make(map[string]int) // ruleID → index in rules slice
	var rules []Rule
	var results []Result

	for _, f := range findings {
		ruleID := safeRuleID(f)

		if _, exists := ruleIndex[ruleID]; !exists {
			ruleIndex[ruleID] = len(rules)
			rules = append(rules, buildRule(ruleID, f))
		}

		results = append(results, buildResult(ruleID, f))
	}

	return Report{
		Version: sarifVersion,
		Schema:  sarifSchema,
		Runs: []Run{{
			Tool:    duckOpsTool(toolVersion, rules),
			Results: results,
			Invocations: []Invocation{{
				ExecutionSuccessful: true,
				StartTimeUTC:        time.Now().UTC(),
				EndTimeUTC:          time.Now().UTC(),
			}},
		}},
	}
}

// duckOpsTool returns the SARIF tool descriptor for DuckOps.
func duckOpsTool(version string, rules []Rule) Tool {
	return Tool{Driver: Driver{
		Name:           "DuckOps",
		Version:        version,
		InformationURI: "https://github.com/SecDuckOps/agent",
		Rules:          rules,
	}}
}

// buildRule creates a SARIF Rule from a Finding.
func buildRule(ruleID string, f domain.Finding) Rule {
	r := Rule{
		ID:               ruleID,
		Name:             sanitize(f.Title),
		ShortDescription: Message{Text: firstLine(f.Title)},
		Properties: &RuleProperties{
			Tags:            []string{strings.ToLower(string(f.Type)), f.Scanner},
			SecuritySeverity: severityScore(f.Severity),
		},
	}
	if f.Description != "" {
		r.FullDescription = &Message{Text: f.Description}
	}
	if f.CVE != "" {
		r.HelpURI = "https://nvd.nist.gov/vuln/detail/" + f.CVE
	}
	return r
}

// buildResult creates a SARIF Result from a Finding.
func buildResult(ruleID string, f domain.Finding) Result {
	res := Result{
		RuleID:  ruleID,
		Level:   severityLevel(f.Severity),
		Message: Message{Text: buildMessage(f)},
	}

	// Resolve file path — prefer Location.File, fall back to Finding.File
	filePath := f.Location.File
	if filePath == "" {
		filePath = f.File
	}
	line := f.Location.Line
	if line == 0 {
		line = f.Line
	}
	col := f.Location.Column

	if filePath != "" {
		uri := toRelativeURI(filePath)
		loc := Location{
			PhysicalLocation: PhysicalLocation{
				ArtifactLocation: ArtifactLocation{URI: uri, URIBaseID: "%SRCROOT%"},
			},
		}
		if line > 0 || col > 0 {
			loc.PhysicalLocation.Region = &Region{
				StartLine:   line,
				StartColumn: col,
			}
		}
		res.Locations = []Location{loc}
	}

	if f.Remediation != "" {
		res.Fixes = []Fix{{Description: Message{Text: f.Remediation}}}
	}

	return res
}

// ── helpers ───────────────────────────────────────────────────────────────────

func safeRuleID(f domain.Finding) string {
	if f.ID != "" {
		return sanitize(f.ID)
	}
	if f.CVE != "" {
		return f.CVE
	}
	return sanitize(f.Scanner + "/" + f.Title)
}

func sanitize(s string) string {
	// Replace characters invalid in SARIF ruleIds
	r := strings.NewReplacer(" ", "-", "/", "-", ":", "-", "\\", "-")
	return r.Replace(strings.TrimSpace(s))
}

func firstLine(s string) string {
	return strings.SplitN(strings.TrimSpace(s), "\n", 2)[0]
}

func toRelativeURI(path string) string {
	// Normalise to forward slashes (SARIF expects URI syntax)
	path = filepath.ToSlash(path)
	// Strip leading ./ or /
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	return path
}

func buildMessage(f domain.Finding) string {
	msg := f.Title
	if f.Description != "" && f.Description != f.Title {
		msg += "\n\n" + f.Description
	}
	if f.CVE != "" {
		msg += fmt.Sprintf("\n\nCVE: %s", f.CVE)
	}
	if f.Match != "" {
		msg += fmt.Sprintf("\n\nMatch: %s", f.Match)
	}
	return msg
}

// severityLevel maps DuckOps severity to SARIF level.
// GitHub uses this to colour-code findings.
func severityLevel(s domain.Severity) string {
	switch s {
	case domain.SeverityCritical, domain.SeverityHigh:
		return "error"
	case domain.SeverityMedium:
		return "warning"
	default:
		return "note"
	}
}

// severityScore returns a CVSS-like 0–10 string for GitHub's security dashboard.
func severityScore(s domain.Severity) string {
	switch s {
	case domain.SeverityCritical:
		return "9.5"
	case domain.SeverityHigh:
		return "7.5"
	case domain.SeverityMedium:
		return "5.0"
	case domain.SeverityLow:
		return "3.0"
	default:
		return "1.0"
	}
}
