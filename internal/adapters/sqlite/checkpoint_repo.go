package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/SecDuckOps/shared/core/session"
)

// implements session.CheckpointRepository for SQLite
type CheckpointRepo struct {
	db *sql.DB
}

func NewCheckpointRepo(db *sql.DB) *CheckpointRepo {
	return &CheckpointRepo{db: db}
}

func (r *CheckpointRepo) Create(ctx context.Context, cp *session.Checkpoint) error {
	query := `
		INSERT INTO checkpoints (
			id, session_id, message_index, summary, created_at, sync_status
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		cp.ID, cp.SessionID, cp.MessageIndex, cp.Summary,
		cp.CreatedAt.UnixNano(), string(cp.SyncStatus),
	)

	return translateErr(err, "creating checkpoint")
}

func (r *CheckpointRepo) ListBySessionID(ctx context.Context, sessionID string) ([]*session.Checkpoint, error) {
	query := `
		SELECT id, session_id, message_index, summary, created_at, sync_status
		FROM checkpoints
		WHERE session_id = ?
		ORDER BY message_index ASC
	`

	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, translateErr(err, "listing checkpoints")
	}
	defer rows.Close()

	var checkpoints []*session.Checkpoint
	for rows.Next() {
		var cp session.Checkpoint
		var createdNano int64

		err := rows.Scan(
			&cp.ID, &cp.SessionID, &cp.MessageIndex, &cp.Summary,
			&createdNano, &cp.SyncStatus,
		)
		if err != nil {
			return nil, translateErr(err, "scanning checkpoint row")
		}

		cp.CreatedAt = time.Unix(0, createdNano)
		checkpoints = append(checkpoints, &cp)
	}

	if err := rows.Err(); err != nil {
		return nil, translateErr(err, "iterating checkpoints")
	}

	return checkpoints, nil
}

func (r *CheckpointRepo) GetLatestBySessionID(ctx context.Context, sessionID string) (*session.Checkpoint, error) {
	query := `
		SELECT id, session_id, message_index, summary, created_at, sync_status
		FROM checkpoints
		WHERE session_id = ?
		ORDER BY message_index DESC
		LIMIT 1
	`

	var cp session.Checkpoint
	var createdNano int64

	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(
		&cp.ID, &cp.SessionID, &cp.MessageIndex, &cp.Summary,
		&createdNano, &cp.SyncStatus,
	)

	if err != nil {
		return nil, translateErr(err, "getting latest checkpoint")
	}

	cp.CreatedAt = time.Unix(0, createdNano)
	return &cp, nil
}
