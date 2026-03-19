package subagents

import (
	"os"
	"path/filepath"
	"strings"
)

// ProjectSignals holds detected signals about a codebase.
// Built by fast filesystem inspection — no LLM needed for this phase.
type ProjectSignals struct {
	Languages  []string // e.g. ["go", "python", "typescript"]
	Frameworks []string // e.g. ["gin", "fastapi", "react"]
	HasIaC     bool     // terraform, k8s manifests, pulumi...
	HasDocker  bool     // Dockerfile, docker-compose
	HasCI      bool     // .github/workflows, .gitlab-ci.yml
	HasTests   bool     // *_test.go, test/, spec/, __tests__/
	FileCount  int
	RootFiles  []string // top-level filenames (max 60)
}

// languageExtensions maps file extensions to language names.
var languageExtensions = map[string]string{
	".go":    "go",
	".py":    "python",
	".ts":    "typescript",
	".tsx":   "typescript",
	".js":    "javascript",
	".jsx":   "javascript",
	".java":  "java",
	".kt":    "kotlin",
	".rb":    "ruby",
	".rs":    "rust",
	".cs":    "csharp",
	".cpp":   "cpp",
	".c":     "c",
	".php":   "php",
	".swift": "swift",
}

// frameworkSignals maps filename patterns to framework names.
var frameworkSignals = map[string]string{
	"go.mod":           "go-modules",
	"requirements.txt": "python",
	"pyproject.toml":   "python",
	"package.json":     "node",
	"pom.xml":          "maven",
	"build.gradle":     "gradle",
	"Gemfile":          "ruby-bundler",
	"Cargo.toml":       "rust-cargo",
	"composer.json":    "php-composer",
}

// iacSignals maps filename/dir patterns to IaC presence.
var iacSignals = []string{
	"main.tf", "variables.tf", "terraform.tfvars",
	"kubernetes", "k8s", "helm", "Chart.yaml",
	"Pulumi.yaml", "serverless.yml", "cdk.json",
	"kustomization.yaml", "manifests",
}

// AnalyzeProject does a fast filesystem walk to collect project signals.
// Intentionally lightweight — stays under 50ms on typical repos.
func AnalyzeProject(targetPath string) ProjectSignals {
	signals := ProjectSignals{}
	langSet := make(map[string]bool)
	fwSet := make(map[string]bool)

	// Walk up to 3 levels deep to keep it fast
	_ = filepath.WalkDir(targetPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden dirs and common noise
		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" ||
				name == "vendor" || name == ".git" || name == "__pycache__" {
				return filepath.SkipDir
			}
			// Depth limit
			rel, _ := filepath.Rel(targetPath, path)
			if strings.Count(rel, string(os.PathSeparator)) >= 3 {
				return filepath.SkipDir
			}
			// IaC directory signals
			lower := strings.ToLower(name)
			for _, sig := range iacSignals {
				if lower == sig {
					signals.HasIaC = true
				}
			}
			return nil
		}

		signals.FileCount++
		lower := strings.ToLower(name)

		// Collect root-level files (max 60)
		rel, _ := filepath.Rel(targetPath, path)
		if !strings.Contains(rel, string(os.PathSeparator)) && len(signals.RootFiles) < 60 {
			signals.RootFiles = append(signals.RootFiles, name)
		}

		// Language detection
		ext := strings.ToLower(filepath.Ext(name))
		if lang, ok := languageExtensions[ext]; ok {
			langSet[lang] = true
		}

		// Framework signals
		if fw, ok := frameworkSignals[name]; ok {
			fwSet[fw] = true
		}

		// IaC file signals
		for _, sig := range iacSignals {
			if lower == sig || strings.HasSuffix(lower, ".tf") ||
				strings.HasSuffix(lower, ".tfvars") {
				signals.HasIaC = true
			}
		}

		// Docker
		if name == "Dockerfile" || name == "docker-compose.yml" ||
			name == "docker-compose.yaml" || strings.HasPrefix(name, "Dockerfile.") {
			signals.HasDocker = true
		}

		// CI/CD
		if strings.Contains(path, ".github/workflows") ||
			name == ".gitlab-ci.yml" || name == "Jenkinsfile" ||
			name == ".circleci" || strings.HasSuffix(name, ".yml") &&
				strings.Contains(path, "ci") {
			signals.HasCI = true
		}

		// Tests
		if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_test.py") ||
			strings.HasSuffix(name, ".spec.ts") || strings.HasSuffix(name, ".test.js") ||
			strings.Contains(path, "/test/") || strings.Contains(path, "/tests/") ||
			strings.Contains(path, "/__tests__/") || strings.Contains(path, "/spec/") {
			signals.HasTests = true
		}

		return nil
	})

	for lang := range langSet {
		signals.Languages = append(signals.Languages, lang)
	}
	for fw := range fwSet {
		signals.Frameworks = append(signals.Frameworks, fw)
	}

	return signals
}

// ScannerRecommendation is the LLM's decision about which scanners to use.
type ScannerRecommendation struct {
	Scanners []string `json:"scanners"` // ordered by priority
	Rationale string  `json:"rationale"`
	SkipReason string `json:"skip_reason,omitempty"` // why some scanners were skipped
}
