package telemetry

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/security"
)

// AuditLogger provides immutable, append-only security audit logging for Warden decisions.
type AuditLogger struct {
	mu       sync.Mutex
	entries  []security.AuditEntry
	filePath string
}

// NewAuditLogger creates a new audit logger. If filePath is provided, entries are persisted.
func NewAuditLogger(filePath string) *AuditLogger {
	return &AuditLogger{
		entries:  make([]security.AuditEntry, 0),
		filePath: filePath,
	}
}

// RecordWardenDecision logs a Warden policy decision as an immutable audit entry.
func (a *AuditLogger) RecordWardenDecision(entry security.AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	a.entries = append(a.entries, entry)

	// Persist to file if configured (append-only)
	if a.filePath != "" {
		return a.appendToFile(entry)
	}
	return nil
}

// GetEntries returns all audit entries (for query/explainability).
func (a *AuditLogger) GetEntries() []security.AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	cpy := make([]security.AuditEntry, len(a.entries))
	copy(cpy, a.entries)
	return cpy
}

// GetByAction filters entries by action type.
func (a *AuditLogger) GetByAction(action security.AuditAction) []security.AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	var result []security.AuditEntry
	for _, e := range a.entries {
		if e.Action == action {
			result = append(result, e)
		}
	}
	return result
}

// appendToFile writes an entry to the audit file in append-only mode.
func (a *AuditLogger) appendToFile(entry security.AuditEntry) error {
	f, err := os.OpenFile(a.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}
