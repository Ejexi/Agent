package agent

import (
	"context"
	"fmt"

	shared_ports "github.com/SecDuckOps/shared/ports"
)

// CloudClientPort is implemented by the cloud push adapter (Phase 4).
// Nil when cloud is not configured — local scan always works without it.
type CloudClientPort interface {
	PushFindings(ctx context.Context, result *AggregatedResult) error
}

// ReportAgent formats scan results for local display and optionally pushes to cloud.
type ReportAgent struct {
	cloudClient CloudClientPort
	logger      shared_ports.Logger
}

// NewReportAgent creates a ReportAgent.
func NewReportAgent(cloudClient CloudClientPort, logger shared_ports.Logger) *ReportAgent {
	return &ReportAgent{
		cloudClient: cloudClient,
		logger:      logger,
	}
}

// Process renders the result via glamour and optionally pushes to cloud.
func (r *ReportAgent) Process(ctx context.Context, result *AggregatedResult) error {
	fmt.Print(Render(FormatMarkdown(result)))

	if r.cloudClient != nil {
		go func() {
			if err := r.cloudClient.PushFindings(ctx, result); err != nil {
				if r.logger != nil {
					r.logger.Info(ctx, fmt.Sprintf("[ReportAgent] cloud push failed (non-fatal): %v", err))
				}
			}
		}()
	}
	return nil
}
