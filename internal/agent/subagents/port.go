package subagents

import (
	"context"

	llm_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/scanner/domain"
	scanner_ports "github.com/SecDuckOps/shared/scanner/ports"
)

// SubagentPort is the interface every scan subagent must implement.
type SubagentPort interface {
	Name() string
	DefaultScanners() []string
	Run(ctx context.Context, targetPath string) (*domain.ScanResult, error)
}

// NewAllSubagents creates all 5 scan subagents wired with the scanner service and LLM.
// llm may be nil — subagents fall back to running all default scanners in that case.
func NewAllSubagents(svc scanner_ports.ScannerServicePort, llm llm_domain.LLM) []SubagentPort {
	return []SubagentPort{
		NewSASTSubagent(svc, llm),
		NewSCASubagent(svc, llm),
		NewSecretsSubagent(svc, llm),
		NewIaCSubagent(svc, llm),
		NewDepsSubagent(svc, llm),
	}
}
