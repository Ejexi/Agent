package sarif_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SecDuckOps/agent/internal/adapters/sarif"
	"github.com/SecDuckOps/shared/scanner/domain"
)

func finding(id, title, severity, file string, line int) domain.Finding {
	return domain.Finding{
		ID:       id,
		Title:    title,
		Severity: domain.Severity(severity),
		Scanner:  "semgrep",
		Type:     domain.ScannerTypeSAST,
		Location: domain.Location{File: file, Line: line},
	}
}

func TestFromFindings_EmptySlice(t *testing.T) {
	r := sarif.FromFindings("1.0.0", nil)
	if r.Version != "2.1.0" {
		t.Fatalf("expected SARIF 2.1.0, got %s", r.Version)
	}
	if len(r.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(r.Runs))
	}
	if len(r.Runs[0].Results) != 0 {
		t.Fatalf("expected 0 results for empty input")
	}
}

func TestFromFindings_ProducesValidJSON(t *testing.T) {
	findings := []domain.Finding{
		finding("SQL-001", "SQL injection", "HIGH", "src/db/query.go", 42),
	}
	r := sarif.FromFindings("1.2.3", findings)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	if !strings.Contains(string(data), "SQL-001") {
		t.Error("expected rule ID in JSON output")
	}
	if !strings.Contains(string(data), "2.1.0") {
		t.Error("expected SARIF version in output")
	}
}

func TestFromFindings_RuleDeduplication(t *testing.T) {
	// Same ID, two occurrences → one rule, two results
	f1 := finding("XSS-001", "XSS", "HIGH", "a.go", 1)
	f2 := finding("XSS-001", "XSS", "HIGH", "b.go", 5)
	r := sarif.FromFindings("", []domain.Finding{f1, f2})
	if len(r.Runs[0].Tool.Driver.Rules) != 1 {
		t.Fatalf("expected 1 deduplicated rule, got %d", len(r.Runs[0].Tool.Driver.Rules))
	}
	if len(r.Runs[0].Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(r.Runs[0].Results))
	}
}

func TestFromFindings_SeverityLevels(t *testing.T) {
	cases := []struct {
		severity string
		level    string
	}{
		{"CRITICAL", "error"},
		{"HIGH", "error"},
		{"MEDIUM", "warning"},
		{"LOW", "note"},
		{"INFO", "note"},
	}

	for _, tc := range cases {
		f := finding("ID-1", "test", tc.severity, "f.go", 1)
		r := sarif.FromFindings("", []domain.Finding{f})
		got := r.Runs[0].Results[0].Level
		if got != tc.level {
			t.Errorf("severity %s → expected level %q, got %q", tc.severity, tc.level, got)
		}
	}
}

func TestFromFindings_LocationMapping(t *testing.T) {
	f := finding("ID-1", "test", "HIGH", "internal/auth/jwt.go", 99)
	r := sarif.FromFindings("", []domain.Finding{f})
	locs := r.Runs[0].Results[0].Locations
	if len(locs) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locs))
	}
	uri := locs[0].PhysicalLocation.ArtifactLocation.URI
	if uri != "internal/auth/jwt.go" {
		t.Errorf("expected clean URI, got %q", uri)
	}
	region := locs[0].PhysicalLocation.Region
	if region == nil || region.StartLine != 99 {
		t.Errorf("expected StartLine 99, got %+v", region)
	}
}

func TestFromFindings_CVELinksToNVD(t *testing.T) {
	f := domain.Finding{
		ID: "CVE-2024-1234", Title: "vuln", Severity: "HIGH",
		Scanner: "trivy", CVE: "CVE-2024-1234",
	}
	r := sarif.FromFindings("", []domain.Finding{f})
	helpURI := r.Runs[0].Tool.Driver.Rules[0].HelpURI
	if !strings.Contains(helpURI, "CVE-2024-1234") {
		t.Errorf("expected NVD link with CVE ID, got %q", helpURI)
	}
}

func TestFromFindings_RemediationInFixes(t *testing.T) {
	f := domain.Finding{
		ID: "ID-1", Title: "SQL injection", Severity: "HIGH",
		Scanner: "semgrep", Remediation: "Use parameterised queries",
	}
	r := sarif.FromFindings("", []domain.Finding{f})
	fixes := r.Runs[0].Results[0].Fixes
	if len(fixes) != 1 || fixes[0].Description.Text != "Use parameterised queries" {
		t.Errorf("expected remediation in fixes, got %+v", fixes)
	}
}

func TestFromFindings_ToolName(t *testing.T) {
	r := sarif.FromFindings("2.0.0", []domain.Finding{finding("A", "x", "LOW", "f.go", 1)})
	if r.Runs[0].Tool.Driver.Name != "DuckOps" {
		t.Errorf("expected tool name 'DuckOps', got %q", r.Runs[0].Tool.Driver.Name)
	}
	if r.Runs[0].Tool.Driver.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got %q", r.Runs[0].Tool.Driver.Version)
	}
}
