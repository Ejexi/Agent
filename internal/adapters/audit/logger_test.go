package audit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SecDuckOps/agent/internal/adapters/audit"
	"github.com/SecDuckOps/agent/internal/domain/security"
	"github.com/SecDuckOps/agent/internal/ports"
)

func tempDirs(t *testing.T) (string, string) {
	t.Helper()
	logDir := filepath.Join(t.TempDir(), "audit")
	backupDir := filepath.Join(t.TempDir(), "backups")
	return logDir, backupDir
}

func TestLogger_RecordAndReplay(t *testing.T) {
	logDir, backupDir := tempDirs(t)
	logger, err := audit.New(logDir, backupDir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	sessionID := "test-session-001"

	entries := []security.AuditEntry{
		{
			SessionID: sessionID,
			Action:    security.AuditSessionStart,
			Actor:     "system",
			Timestamp: time.Now(),
		},
		{
			SessionID: sessionID,
			Action:    security.AuditToolExecute,
			Actor:     "echo",
			Target:    "echo tool",
			Details:   map[string]interface{}{"message": "hello"},
			Timestamp: time.Now(),
		},
		{
			SessionID: sessionID,
			Action:    security.AuditToolResult,
			Actor:     "echo",
			Details:   map[string]interface{}{"success": true},
			Timestamp: time.Now(),
		},
	}

	for _, e := range entries {
		if err := logger.Record(ctx, e); err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	// Replay
	replayed, err := logger.ReplaySession(ctx, sessionID)
	if err != nil {
		t.Fatalf("ReplaySession failed: %v", err)
	}

	if len(replayed) != len(entries) {
		t.Errorf("expected %d entries, got %d", len(entries), len(replayed))
	}

	// Check JSONL file exists
	jsonlPath := filepath.Join(logDir, sessionID+".jsonl")
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		t.Error("JSONL file not created")
	}
}

func TestLogger_QueryByAction(t *testing.T) {
	logDir, backupDir := tempDirs(t)
	logger, err := audit.New(logDir, backupDir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	sessionID := "query-session"

	// Record mixed actions
	logger.Record(ctx, security.AuditEntry{SessionID: sessionID, Action: security.AuditToolExecute, Actor: "scan", Timestamp: time.Now()})
	logger.Record(ctx, security.AuditEntry{SessionID: sessionID, Action: security.AuditToolResult, Actor: "scan", Timestamp: time.Now()})
	logger.Record(ctx, security.AuditEntry{SessionID: sessionID, Action: security.AuditFileEdit, Actor: "system", Timestamp: time.Now()})
	logger.Record(ctx, security.AuditEntry{SessionID: sessionID, Action: security.AuditToolExecute, Actor: "echo", Timestamp: time.Now()})

	// Query only tool.execute actions
	results, err := logger.Query(ctx, ports.AuditFilter{
		SessionID: sessionID,
		Action:    security.AuditToolExecute,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 tool.execute entries, got %d", len(results))
	}
}

func TestLogger_BackupSession(t *testing.T) {
	logDir, backupDir := tempDirs(t)
	logger, err := audit.New(logDir, backupDir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	sessionID := "backup-session"

	snapshot := security.SessionSnapshot{
		SessionID: sessionID,
		Files: map[string][]byte{
			"/tmp/test.go":   []byte("package main"),
			"/tmp/config.go": []byte("package config"),
		},
		FileList:  []string{"/tmp/test.go", "/tmp/config.go"},
		CreatedAt: time.Now(),
	}

	if err := logger.BackupSession(ctx, snapshot); err != nil {
		t.Fatalf("BackupSession failed: %v", err)
	}

	// Verify manifest exists
	manifestPath := filepath.Join(backupDir, sessionID, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("manifest.json not created")
	}
}

func TestLogger_QueryWithLimit(t *testing.T) {
	logDir, backupDir := tempDirs(t)
	logger, err := audit.New(logDir, backupDir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	sessionID := "limit-session"

	for i := 0; i < 10; i++ {
		logger.Record(ctx, security.AuditEntry{
			SessionID: sessionID,
			Action:    security.AuditToolExecute,
			Actor:     "tool",
			Timestamp: time.Now(),
		})
	}

	results, err := logger.Query(ctx, ports.AuditFilter{
		SessionID: sessionID,
		Limit:     3,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 entries with limit, got %d", len(results))
	}
}
