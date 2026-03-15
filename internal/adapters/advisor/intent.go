package advisor

import (
	"context"
	"strings"

	"github.com/SecDuckOps/agent/internal/domain"
)

// destructiveActions maps known high-risk actions to their risk scores.
var destructiveActions = map[string]float64{
	"delete":  0.9,
	"destroy": 1.0,
	"rm":      0.8,
	"drop":    1.0,
	"purge":   0.9,
	"stop":    0.5,
	"kill":    0.6,
	"reset":   0.7,
	"format":  1.0,
}

// destructiveFlags are flags that amplify risk.
var destructiveFlags = map[string]float64{
	"--force":       0.2,
	"-f":            0.2,
	"--recursive":   0.15,
	"-r":            0.15,
	"--all":         0.1,
	"--no-preserve": 0.15,
	"--cascade":     0.2,
}

// criticalResources are resource patterns that indicate high-impact targets.
var criticalResources = []string{
	"/", "/etc", "/var", "/usr", "/root",
	"production", "prod", "main",
	"database", "db", "master",
}

// IntentDetector provides fast, local destructive intent analysis.
// This runs WITHOUT calling any LLM — it is purely heuristic-based.
type IntentDetector struct{}

// NewIntentDetector creates a new IntentDetector.
func NewIntentDetector() *IntentDetector {
	return &IntentDetector{}
}

// DetectDestructiveIntent performs fast-path local detection of destructive commands.
func (d *IntentDetector) DetectDestructiveIntent(_ context.Context, ast domain.ExecutionAST) (float64, string, error) {
	baseRisk := 0.0
	explanation := ""

	// Check base action risk
	action := strings.ToLower(ast.Action)
	if risk, found := destructiveActions[action]; found {
		baseRisk = risk
		explanation = "🚨 Destructive action detected: " + ast.Action
	}

	if baseRisk == 0.0 {
		return 0.0, " Action appears safe", nil
	}

	// Amplify risk based on flags
	flagRisk := 0.0
	for flag := range ast.Flags {
		if amp, found := destructiveFlags[flag]; found {
			flagRisk += amp
			explanation += " | Risky flag: " + flag
		}
	}

	// Check resource criticality
	resourceRisk := 0.0
	resource := strings.ToLower(ast.Resource)
	for _, critical := range criticalResources {
		if strings.Contains(resource, critical) {
			resourceRisk = 0.3
			explanation += " | Critical resource target: " + ast.Resource
			break
		}
	}

	// Composite risk (capped at 1.0)
	totalRisk := baseRisk + flagRisk + resourceRisk
	if totalRisk > 1.0 {
		totalRisk = 1.0
	}

	return totalRisk, explanation, nil
}
