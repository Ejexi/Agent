package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SecDuckOps/agent/internal/adapters/sarif"
	shared_ports "github.com/SecDuckOps/shared/ports"
	"github.com/SecDuckOps/shared/types"
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

// Process renders the result according to outputFmt:
//   - ""       → glamour-rendered Markdown (default)
//   - "text"   → same as ""
//   - "json"   → raw JSON to stdout
//   - "sarif"  → SARIF 2.1.0 JSON to stdout (for CI/GitHub Actions)
func (r *ReportAgent) Process(ctx context.Context, result *AggregatedResult, outputFmt string) error {
	switch outputFmt {
	case "sarif":
		return r.writeSARIF(result)
	case "json":
		return r.writeJSON(result)
	default:
		fmt.Print(Render(FormatMarkdown(result)))
	}

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

func (r *ReportAgent) writeSARIF(result *AggregatedResult) error {
	report := sarif.FromFindings("", result.AllFindings)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "sarif: marshal failed")
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}

func (r *ReportAgent) writeJSON(result *AggregatedResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "json: marshal failed")
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}
