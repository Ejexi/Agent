package subagents

import (
	"context"
	"fmt"
	"strings"

	llm_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/scanner/domain"
	scanner_ports "github.com/SecDuckOps/shared/scanner/ports"
)

// ── SAST Subagent ──────────────────────────────────────────────────────────

const sastSystemPrompt = `You are a senior application security engineer specialising in Static Application Security Testing (SAST).

Your mission:
- Understand the codebase language and framework stack
- Select ONLY the SAST scanners relevant to the detected languages
- After scanning, identify the highest-risk code patterns: injection vectors, unsafe deserialization, authentication bypasses, hardcoded logic flaws
- Correlate findings across scanners to eliminate duplicates
- Prioritize exploitable findings over theoretical ones

Scanner guidance:
- semgrep: universal, good for all languages — always include
- gosec: Go-specific — only if Go is detected
- bandit: Python-specific — only if Python is detected
- njsscan: Node.js/JS-specific — only if JS/TS is detected
- brakeman: Ruby on Rails — only if Ruby is detected`

type SASTSubagent struct {
	intelligentBase
}

func NewSASTSubagent(svc scanner_ports.ScannerServicePort, llm llm_domain.LLM) *SASTSubagent {
	return &SASTSubagent{intelligentBase{
		scannerSvc:      svc,
		llm:             llm,
		defaultScanners: []string{"semgrep", "gosec", "bandit", "njsscan", "brakeman"},
		categoryName:    "sast",
		systemPrompt:    sastSystemPrompt,
	}}
}

func (s *SASTSubagent) Name() string             { return "sast" }
func (s *SASTSubagent) DefaultScanners() []string { return s.defaultScanners }

func (s *SASTSubagent) Run(ctx context.Context, targetPath string) (*domain.ScanResult, error) {
	findings, insight, err := s.runIntelligent(ctx, targetPath)
	return buildResult(s.Name(), findings, insight), err
}

// ── SCA Subagent ───────────────────────────────────────────────────────────

const scaSystemPrompt = `You are a senior supply chain security engineer specialising in Software Composition Analysis (SCA).

Your mission:
- Identify the dependency ecosystem (Go modules, pip, npm, maven, cargo...)
- Select ONLY scanners that understand the detected package managers
- Focus on: known CVEs in direct dependencies, transitive dependency risks, outdated packages with exploits, license risks
- Flag supply chain risks: typosquatting patterns, suspicious package names, maintainer abandonment signals

Scanner guidance:
- trivy: excellent general-purpose — always include for containers + filesystem
- grype: strong CVE database, good for containers
- osvscanner: Google's OSV database — good for most ecosystems
- dependencycheck: OWASP, strong for Java/Maven/Gradle`

type SCASubagent struct {
	intelligentBase
}

func NewSCASubagent(svc scanner_ports.ScannerServicePort, llm llm_domain.LLM) *SCASubagent {
	return &SCASubagent{intelligentBase{
		scannerSvc:      svc,
		llm:             llm,
		defaultScanners: []string{"trivy", "grype", "osvscanner", "dependencycheck"},
		categoryName:    "sca",
		systemPrompt:    scaSystemPrompt,
	}}
}

func (s *SCASubagent) Name() string             { return "sca" }
func (s *SCASubagent) DefaultScanners() []string { return s.defaultScanners }

func (s *SCASubagent) Run(ctx context.Context, targetPath string) (*domain.ScanResult, error) {
	findings, insight, err := s.runIntelligent(ctx, targetPath)
	return buildResult(s.Name(), findings, insight), err
}

// ── Secrets Subagent ───────────────────────────────────────────────────────

const secretsSystemPrompt = `You are a senior secrets detection engineer.

Your mission:
- Detect hardcoded secrets, API keys, tokens, certificates, and credentials
- Identify high-risk patterns: cloud provider keys (AWS, GCP, Azure), database URLs with credentials, private keys, JWT secrets
- Distinguish between real secrets and test/example values (reduce false positives)
- Flag secrets in: source code, config files, CI/CD definitions, IaC files, commit history signals

Scanner guidance:
- gitleaks: excellent regex-based detection — always include
- trufflehog: entropy-based + known patterns — always include
- detectsecrets: Yelp's tool, good for whitelisting known false positives

All three scanners are relevant regardless of language — always run all.`

type SecretsSubagent struct {
	intelligentBase
}

func NewSecretsSubagent(svc scanner_ports.ScannerServicePort, llm llm_domain.LLM) *SecretsSubagent {
	return &SecretsSubagent{intelligentBase{
		scannerSvc:      svc,
		llm:             llm,
		defaultScanners: []string{"gitleaks", "trufflehog", "detectsecrets"},
		categoryName:    "secrets",
		systemPrompt:    secretsSystemPrompt,
	}}
}

func (s *SecretsSubagent) Name() string             { return "secrets" }
func (s *SecretsSubagent) DefaultScanners() []string { return s.defaultScanners }

func (s *SecretsSubagent) Run(ctx context.Context, targetPath string) (*domain.ScanResult, error) {
	findings, insight, err := s.runIntelligent(ctx, targetPath)
	return buildResult(s.Name(), findings, insight), err
}

// ── IaC Subagent ───────────────────────────────────────────────────────────

const iacSystemPrompt = `You are a senior cloud infrastructure security engineer specialising in Infrastructure-as-Code (IaC) security.

Your mission:
- Detect IaC technology stack (Terraform, Kubernetes, Helm, Pulumi, CDK, serverless)
- Select ONLY scanners relevant to the detected IaC technology
- Focus on: overly permissive IAM policies, public exposure of sensitive resources, unencrypted storage, missing network segmentation, privilege escalation paths
- Flag misconfigurations that lead to: data exposure, lateral movement, resource abuse

Scanner guidance:
- checkov: broad coverage (Terraform, K8s, CloudFormation, ARM) — use when IaC is present
- tfsec: Terraform-specific deep analysis — use only when .tf files detected
- kics: broad IaC scanner — use for multi-cloud
- terrascan: good for policy-as-code
- tflint: Terraform linting — use with tfsec

If NO IaC files are detected, return empty scanners array with a skip_reason.`

type IaCSubagent struct {
	intelligentBase
}

func NewIaCSubagent(svc scanner_ports.ScannerServicePort, llm llm_domain.LLM) *IaCSubagent {
	return &IaCSubagent{intelligentBase{
		scannerSvc:      svc,
		llm:             llm,
		defaultScanners: []string{"checkov", "tfsec", "kics", "terrascan", "tflint"},
		categoryName:    "iac",
		systemPrompt:    iacSystemPrompt,
	}}
}

func (s *IaCSubagent) Name() string             { return "iac" }
func (s *IaCSubagent) DefaultScanners() []string { return s.defaultScanners }

func (s *IaCSubagent) Run(ctx context.Context, targetPath string) (*domain.ScanResult, error) {
	findings, insight, err := s.runIntelligent(ctx, targetPath)
	return buildResult(s.Name(), findings, insight), err
}

// ── Deps Subagent ──────────────────────────────────────────────────────────

const depsSystemPrompt = `You are a senior dependency security engineer.

Your mission:
- Identify outdated dependencies with known exploits
- Flag transitive dependency risks (indirect dependencies introducing CVEs)
- Detect license compliance risks
- Identify abandoned packages (no updates in 2+ years with active CVEs)

Scanner guidance:
- osvscanner: Google OSV — strong for Go, Python, npm, Maven
- dependencycheck: OWASP NVD — comprehensive for Java/Node

Select based on detected package managers. If no dependency manifests exist, skip.`

type DepsSubagent struct {
	intelligentBase
}

func NewDepsSubagent(svc scanner_ports.ScannerServicePort, llm llm_domain.LLM) *DepsSubagent {
	return &DepsSubagent{intelligentBase{
		scannerSvc:      svc,
		llm:             llm,
		defaultScanners: []string{"osvscanner", "dependencycheck"},
		categoryName:    "deps",
		systemPrompt:    depsSystemPrompt,
	}}
}

func (s *DepsSubagent) Name() string             { return "deps" }
func (s *DepsSubagent) DefaultScanners() []string { return s.defaultScanners }

func (s *DepsSubagent) Run(ctx context.Context, targetPath string) (*domain.ScanResult, error) {
	findings, insight, err := s.runIntelligent(ctx, targetPath)
	return buildResult(s.Name(), findings, insight), err
}

// ── DAST Subagent (placeholder) ────────────────────────────────────────────

type DASTSubagent struct {
	intelligentBase
}

func NewDASTSubagent(svc scanner_ports.ScannerServicePort, llm llm_domain.LLM) *DASTSubagent {
	return &DASTSubagent{intelligentBase{
		scannerSvc:      svc,
		llm:             llm,
		defaultScanners: []string{"zap", "nuclei"},
		categoryName:    "dast",
		systemPrompt:    "You are a DAST specialist. DAST requires a live running target — skip filesystem analysis.",
	}}
}

func (s *DASTSubagent) Name() string             { return "dast" }
func (s *DASTSubagent) DefaultScanners() []string { return s.defaultScanners }

func (s *DASTSubagent) Run(ctx context.Context, targetPath string) (*domain.ScanResult, error) {
	return buildResult(s.Name(), nil, "DAST requires a live target URL — skipped for filesystem scan."), nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

// buildResult constructs a ScanResult with an optional LLM insight appended.
func buildResult(name string, findings []domain.Finding, insight string) *domain.ScanResult {
	r := &domain.ScanResult{
		ScannerName: name,
		Findings:    findings,
	}
	if insight != "" {
		// Prepend insight to RawOutput so it surfaces in reports
		r.RawOutput = fmt.Sprintf("## AI Security Insight\n\n%s\n\n---\n", insight)
	}
	return r
}

// insightLines formats an insight string as indented lines for display.
func insightLines(insight string) string {
	if insight == "" {
		return ""
	}
	lines := strings.Split(insight, "\n")
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString("  " + l + "\n")
	}
	return sb.String()
}

var _ = insightLines // suppress unused warning
