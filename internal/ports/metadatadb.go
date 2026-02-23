package ports

import (
	"context"
	"github.com/SecDuckOps/agent/internal/domain"
)

// MetadataDB defines the interface for the vulnerability metadata store (PostgreSQL).
// Stores structured scan results: vulnerabilities, statuses, summaries.
type MetadataDB interface {
	// SaveScanResult persists a completed scan result.
	SaveScanResult(ctx context.Context, result *domain.ScanResult) error

	// GetScanResult retrieves a scan result by its scan ID.
	GetScanResult(ctx context.Context, scanID string) (*domain.ScanResult, error)

	// ListScanResults returns scan results matching the given filter criteria.
	ListScanResults(ctx context.Context, filter ScanResultFilter) ([]*domain.ScanResult, error)

	// GetVulnerabilities retrieves all vulnerabilities for a given scan ID.
	GetVulnerabilities(ctx context.Context, scanID string) ([]domain.Vulnerability, error)

	// GetVulnerabilityByID retrieves a single vulnerability by its ID.
	GetVulnerabilityByID(ctx context.Context, vulnID string) (*domain.Vulnerability, error)

	// CountBySeverity returns the number of vulnerabilities grouped by severity,
	// optionally filtered to a specific scan.
	CountBySeverity(ctx context.Context, scanID string) (map[domain.Severity]int, error)

	// UpdateScanStatus transitions the status of a scan (e.g., PENDING → RUNNING → COMPLETED).
	UpdateScanStatus(ctx context.Context, scanID string, status domain.ScanStatus) error

	// Close gracefully shuts down the database connection.
	Close() error
}

// ScanResultFilter provides criteria for querying scan results.
type ScanResultFilter struct {
	ScannerType *domain.ScannerType `json:"scanner_type,omitempty"`
	Status      *domain.ScanStatus  `json:"status,omitempty"`
	Target      string              `json:"target,omitempty"`
	Limit       int                 `json:"limit,omitempty"`
	Offset      int                 `json:"offset,omitempty"`
}
