// Package checkpoint persists session message history to disk so sessions
// can be resumed after the process exits.
//
// Storage: ~/.duckops/sessions/<session_id>.json (one file per session)
// Format: JSON — CheckpointEnvelope wrapping []Message + metadata
//
// Mirrors duckops cli/src/commands/agent/run/checkpoint.rs
package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	shared_domain "github.com/SecDuckOps/shared/llm/domain"
	"github.com/SecDuckOps/shared/types"
)

// CheckpointEnvelope is the on-disk format.
type CheckpointEnvelope struct {
	Version   int                     `json:"version"`
	SessionID string                  `json:"session_id"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
	Messages  []shared_domain.Message `json:"messages"`
	Metadata  map[string]interface{}  `json:"metadata,omitempty"`
}

const envelopeVersion = 1

// Store persists checkpoints to ~/.duckops/sessions/.
type Store struct {
	dir string
}

// NewStore creates a Store using the given directory.
// Use config.DuckOpsDir() + "/sessions" as the standard path.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "checkpoint: failed to create sessions directory")
	}
	return &Store{dir: dir}, nil
}

// Save persists the current message history for a session.
// Creates a new file if none exists; overwrites if it does.
func (s *Store) Save(_ context.Context, sessionID string, messages []shared_domain.Message, meta map[string]interface{}) error {
	path := s.path(sessionID)

	env := CheckpointEnvelope{
		Version:   envelopeVersion,
		SessionID: sessionID,
		UpdatedAt: time.Now().UTC(),
		Messages:  messages,
		Metadata:  meta,
	}

	// Preserve CreatedAt if file already exists
	if existing, err := s.Load(context.Background(), sessionID); err == nil {
		env.CreatedAt = existing.CreatedAt
	} else {
		env.CreatedAt = env.UpdatedAt
	}

	b, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "checkpoint: marshal failed")
	}
	// Write atomically via temp file
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "checkpoint: write failed")
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return types.Wrap(err, types.ErrCodeInternal, "checkpoint: rename failed")
	}
	return nil
}

// Load reads and returns the checkpoint for a session.
func (s *Store) Load(_ context.Context, sessionID string) (*CheckpointEnvelope, error) {
	b, err := os.ReadFile(s.path(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, types.Newf(types.ErrCodeNotFound, "no checkpoint for session %s", sessionID)
		}
		return nil, types.Wrap(err, types.ErrCodeInternal, "checkpoint: read failed")
	}
	var env CheckpointEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "checkpoint: unmarshal failed")
	}
	return &env, nil
}

// Delete removes the checkpoint file for a session.
func (s *Store) Delete(_ context.Context, sessionID string) error {
	err := os.Remove(s.path(sessionID))
	if err != nil && !os.IsNotExist(err) {
		return types.Wrap(err, types.ErrCodeInternal, "checkpoint: delete failed")
	}
	return nil
}

// ListSessions returns all session IDs that have a saved checkpoint,
// sorted by last-updated time (most recent first).
func (s *Store) ListSessions() ([]CheckpointEnvelope, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "checkpoint: list failed")
	}

	var results []CheckpointEnvelope
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var env CheckpointEnvelope
		if err := json.Unmarshal(b, &env); err != nil {
			continue
		}
		results = append(results, env)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})
	return results, nil
}

// Exists reports whether a checkpoint file exists for the session.
func (s *Store) Exists(sessionID string) bool {
	_, err := os.Stat(s.path(sessionID))
	return err == nil
}

func (s *Store) path(sessionID string) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s.json", sessionID))
}
