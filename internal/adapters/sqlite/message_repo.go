package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/SecDuckOps/shared/core/session"
)

// implements session.MessageRepository for SQLite
type MessageRepo struct {
	db *sql.DB
}

func NewMessageRepo(db *sql.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

func (r *MessageRepo) Create(ctx context.Context, m *session.Message) error {
	query := `
		INSERT INTO messages (
			id, session_id, role, content, created_at, sync_status
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		m.ID, m.SessionID, string(m.Role), m.Content,
		m.CreatedAt.UnixNano(), string(m.SyncStatus),
	)

	return translateErr(err, "creating message")
}

func (r *MessageRepo) ListBySessionID(ctx context.Context, sessionID string, offset, limit int) ([]*session.Message, error) {
	query := `
		SELECT id, session_id, role, content, created_at, sync_status
		FROM messages
		WHERE session_id = ?
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, sessionID, limit, offset)
	if err != nil {
		return nil, translateErr(err, "listing messages")
	}
	defer rows.Close()

	var messages []*session.Message
	for rows.Next() {
		var m session.Message
		var createdNano int64

		err := rows.Scan(
			&m.ID, &m.SessionID, &m.Role, &m.Content, &createdNano, &m.SyncStatus,
		)
		if err != nil {
			return nil, translateErr(err, "scanning message row")
		}

		m.CreatedAt = time.Unix(0, createdNano)
		messages = append(messages, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, translateErr(err, "iterating messages")
	}

	return messages, nil
}

func (r *MessageRepo) CountBySessionID(ctx context.Context, sessionID string) (int, error) {
	query := `SELECT COUNT(*) FROM messages WHERE session_id = ?`

	var count int
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(&count)
	if err != nil {
		return 0, translateErr(err, "counting messages")
	}

	return count, nil
}
