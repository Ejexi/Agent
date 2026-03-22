package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/SecDuckOps/shared/core/session"
	"github.com/SecDuckOps/shared/types"
)

// implements session.SessionRepository for SQLite
type SessionRepo struct {
	db *sql.DB
}

// Constructor give the repo db to work
func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create a new session
func (r *SessionRepo) Create(ctx context.Context, s *session.Session) error {
	// marshal the session metadata to json
	metaJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to marshal session metadata")
	}

	query := `
		INSERT INTO sessions (
			id, user_id, device_id, name, status, metadata, 
			created_at, updated_at, deleted_at, version, sync_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var deletedAtNano sql.NullInt64
	if s.DeletedAt != nil {
		deletedAtNano.Valid = true
		deletedAtNano.Int64 = s.DeletedAt.UnixNano()
	}

	_, err = r.db.ExecContext(ctx, query,
		s.ID, s.UserID, s.DeviceID, s.Name, s.Status, string(metaJSON),
		s.CreatedAt.UnixNano(), s.UpdatedAt.UnixNano(), deletedAtNano,
		s.Version, s.SyncStatus,
	)

	return translateErr(err, "creating session")
}

func (r *SessionRepo) GetByID(ctx context.Context, id string) (*session.Session, error) {
	query := `
		SELECT 
			id, user_id, device_id, name, status, metadata, 
			created_at, updated_at, deleted_at, version, sync_status
		FROM sessions 
		WHERE id = ? AND deleted_at IS NULL
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanSession(row)
}

func (r *SessionRepo) List(ctx context.Context, userID string, offset, limit int) ([]*session.Session, error) {
	query := `
		SELECT 
			id, user_id, device_id, name, status, metadata, 
			created_at, updated_at, deleted_at, version, sync_status
		FROM sessions 
		WHERE user_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, translateErr(err, "listing sessions")
	}
	defer rows.Close()

	var sessions []*session.Session
	for rows.Next() {
		s, err := r.scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	if err := rows.Err(); err != nil {
		return nil, translateErr(err, "iterating sessions")
	}

	return sessions, nil
}

func (r *SessionRepo) Update(ctx context.Context, s *session.Session) error {
	metaJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to marshal session metadata")
	}

	query := `
		UPDATE sessions SET 
			name = ?, status = ?, metadata = ?, 
			updated_at = ?, version = ?, sync_status = ?
		WHERE id = ? AND version = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		s.Name, s.Status, string(metaJSON),
		s.UpdatedAt.UnixNano(), s.Version, s.SyncStatus,
		s.ID, s.Version-1, // Optimistic lock
	)

	if err != nil {
		return translateErr(err, "updating session")
	}

	return checkRowsAffected(result, session.ErrVersionConflict, "updating session")
}

func (r *SessionRepo) SoftDelete(ctx context.Context, id string) error {
	query := `
		UPDATE sessions SET 
			status = 'deleted', deleted_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, time.Now().UnixNano(), id)
	if err != nil {
		return translateErr(err, "soft deleting session")
	}

	return checkRowsAffected(result, session.ErrNotFound, "soft deleting session")
}

// scanSession handles reading a single session from a sql.Row or sql.Rows
func (r *SessionRepo) scanSession(scanner interface {
	Scan(dest ...any) error
}) (*session.Session, error) {
	var s session.Session
	var metaJSON string
	var createdNano, updatedNano int64
	var deletedNano sql.NullInt64

	err := scanner.Scan(
		&s.ID, &s.UserID, &s.DeviceID, &s.Name, &s.Status, &metaJSON,
		&createdNano, &updatedNano, &deletedNano,
		&s.Version, &s.SyncStatus,
	)

	if err != nil {
		return nil, translateErr(err, "scanning session row")
	}

	if err := json.Unmarshal([]byte(metaJSON), &s.Metadata); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to unmarshal session metadata")
	}

	s.CreatedAt = time.Unix(0, createdNano)
	s.UpdatedAt = time.Unix(0, updatedNano)
	if deletedNano.Valid {
		t := time.Unix(0, deletedNano.Int64)
		s.DeletedAt = &t
	}

	return &s, nil
}
