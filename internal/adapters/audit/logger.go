package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/shared/types"
	"github.com/google/uuid"
)

// Logger implements ports.AuditLogPort using JSONL files.
// One file per session: ~/.duckops/audit/{session_id}.jsonl
// Backups stored in: ~/.duckops/audit/backups/{session_id}/
type Logger struct {
	logDir    string
	backupDir string
	mu        sync.Mutex
	writers   map[string]*os.File // session_id → open file handle
}

// New creates a new audit Logger.
func New(logDir, backupDir string) (*Logger, error) {
	if logDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, types.Wrap(err, types.ErrCodeInternal, "cannot determine home directory")
		}
		logDir = filepath.Join(home, ".duckops", "audit")
	}
	if backupDir == "" {
		backupDir = filepath.Join(logDir, "backups")
	}

	// Ensure directories exist
	for _, dir := range []string{logDir, backupDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, types.Wrapf(err, types.ErrCodeInternal, "cannot create directory %s", dir)
		}
	}

	return &Logger{
		logDir:    logDir,
		backupDir: backupDir,
		writers:   make(map[string]*os.File),
	}, nil
}

// Record writes an immutable audit entry to the session's JSONL file.
func (l *Logger) Record(_ context.Context, entry security.AuditEntry) error {
	// Assign ID and timestamp if missing
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to marshal audit entry")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := l.getOrCreateWriter(entry.SessionID)
	if err != nil {
		return err
	}

	// Append JSONL line
	if _, err := f.Write(append(data, '\n')); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to write audit entry")
	}

	return f.Sync()
}

// Query returns audit entries matching the filter.
func (l *Logger) Query(_ context.Context, filter ports.AuditFilter) ([]security.AuditEntry, error) {
	var entries []security.AuditEntry

	// If session ID specified, read only that file
	if filter.SessionID != "" {
		sessionEntries, err := l.readSessionFile(filter.SessionID)
		if err != nil {
			return nil, err
		}
		entries = filterEntries(sessionEntries, filter)
		return entries, nil
	}

	// Otherwise scan all session files
	files, err := os.ReadDir(l.logDir)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to read audit directory")
	}

	for _, f := range files {
		if f.IsDir() || filepath.Ext(f.Name()) != ".jsonl" {
			continue
		}
		sessionID := f.Name()[:len(f.Name())-len(".jsonl")]
		sessionEntries, err := l.readSessionFile(sessionID)
		if err != nil {
			continue // skip unreadable files
		}
		entries = append(entries, filterEntries(sessionEntries, filter)...)
	}

	return entries, nil
}

// BackupSession creates a snapshot backup of all files modified in the session.
func (l *Logger) BackupSession(_ context.Context, snapshot security.SessionSnapshot) error {
	sessionBackupDir := filepath.Join(l.backupDir, snapshot.SessionID)
	if err := os.MkdirAll(sessionBackupDir, 0700); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "cannot create backup directory")
	}

	for filePath, content := range snapshot.Files {
		// Flatten the file path to a safe filename
		safeName := flattenPath(filePath)
		backupPath := filepath.Join(sessionBackupDir, safeName)

		if err := os.WriteFile(backupPath, content, 0600); err != nil {
			return types.Wrapf(err, types.ErrCodeInternal, "failed to backup file %s", filePath)
		}
	}

	// Write a manifest
	manifest := struct {
		SessionID string    `json:"session_id"`
		Files     []string  `json:"files"`
		CreatedAt time.Time `json:"created_at"`
	}{
		SessionID: snapshot.SessionID,
		Files:     snapshot.FileList,
		CreatedAt: snapshot.CreatedAt,
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	manifestPath := filepath.Join(sessionBackupDir, "manifest.json")
	return os.WriteFile(manifestPath, manifestData, 0600)
}

// ReplaySession returns all audit entries for a session in chronological order.
func (l *Logger) ReplaySession(_ context.Context, sessionID string) ([]security.AuditEntry, error) {
	return l.readSessionFile(sessionID)
}

// Close gracefully shuts down all open file handles.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var lastErr error
	for id, f := range l.writers {
		if err := f.Close(); err != nil {
			lastErr = err
		}
		delete(l.writers, id)
	}
	return lastErr
}

// ──────────────── internal helpers ────────────────

func (l *Logger) getOrCreateWriter(sessionID string) (*os.File, error) {
	if f, ok := l.writers[sessionID]; ok {
		return f, nil
	}

	path := filepath.Join(l.logDir, sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "cannot open audit file")
	}

	l.writers[sessionID] = f
	return f, nil
}

func (l *Logger) readSessionFile(sessionID string) ([]security.AuditEntry, error) {
	path := filepath.Join(l.logDir, sessionID+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "cannot open session file")
	}
	defer f.Close()

	var entries []security.AuditEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry security.AuditEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

func filterEntries(entries []security.AuditEntry, filter ports.AuditFilter) []security.AuditEntry {
	var result []security.AuditEntry

	for _, e := range entries {
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		if filter.Actor != "" && e.Actor != filter.Actor {
			continue
		}
		if filter.From != nil && e.Timestamp.Before(*filter.From) {
			continue
		}
		if filter.To != nil && e.Timestamp.After(*filter.To) {
			continue
		}
		result = append(result, e)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}

	return result
}

// flattenPath converts a file path to a safe filename for backup storage.
func flattenPath(path string) string {
	// Replace path separators and colons with underscores
	safe := filepath.ToSlash(path)
	replacer := func(r rune) rune {
		if r == '/' || r == ':' || r == '\\' {
			return '_'
		}
		return r
	}
	result := make([]rune, 0, len(safe))
	for _, r := range safe {
		result = append(result, replacer(r))
	}
	return string(result)
}
