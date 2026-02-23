package ports

import (
	"context"
	"time"
)

// LogDB defines the interface for the raw scan log store (Elasticsearch).
// Optimized for high-volume, append-heavy log ingestion and full-text search.
type LogDB interface {
	// StoreLogs persists raw scan output logs associated with a scan ID.
	StoreLogs(ctx context.Context, scanID string, logs []string) error

	// GetLogs retrieves all stored logs for a given scan ID.
	GetLogs(ctx context.Context, scanID string) ([]LogEntry, error)

	// SearchLogs performs a full-text search across all stored scan logs.
	SearchLogs(ctx context.Context, query LogSearchQuery) ([]LogEntry, error)

	// DeleteLogs removes all log entries for a given scan ID (e.g., retention policy).
	DeleteLogs(ctx context.Context, scanID string) error

	// Close gracefully shuts down the connection to the log store.
	Close() error
}

// LogEntry represents a single log line stored in the log database.
type LogEntry struct {
	ScanID    string    `json:"scan_id"`
	Line      string    `json:"line"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level,omitempty"` // e.g., INFO, WARN, ERROR
}

// LogSearchQuery provides criteria for searching scan logs.
type LogSearchQuery struct {
	Text   string     `json:"text"`              // Full-text search term
	ScanID string     `json:"scan_id,omitempty"` // Scope search to a specific scan
	Level  string     `json:"level,omitempty"`   // Filter by log level
	From   *time.Time `json:"from,omitempty"`    // Start of time range
	To     *time.Time `json:"to,omitempty"`      // End of time range
	Limit  int        `json:"limit,omitempty"`
	Offset int        `json:"offset,omitempty"`
}
