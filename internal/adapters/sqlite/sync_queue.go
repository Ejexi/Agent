package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/SecDuckOps/shared/core/session"
)

type SyncQueueRepo struct {
	db *sql.DB
}

func NewSyncQueueRepo(db *sql.DB) *SyncQueueRepo {
	return &SyncQueueRepo{db: db}
}

func (r *SyncQueueRepo) Enqueue(ctx context.Context, item session.SyncItem) error {
	query := `
		INSERT INTO sync_queue (
			id, table_name, record_id, operation, attempts, created_at, last_error
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		item.ID, item.TableName, item.RecordID, item.Operation,
		item.Attempts, item.CreatedAt.UnixNano(), item.LastError,
	)

	return translateErr(err, "enqueueing sync item")
}

// if we add MULTIPLE background workers we need to use the locking <future>
func (r *SyncQueueRepo) Dequeue(ctx context.Context, limit int) ([]session.SyncItem, error) {
	query := `
		SELECT id, table_name, record_id, operation, attempts, created_at, last_error
		FROM sync_queue
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, translateErr(err, "dequeueing sync items")
	}
	defer rows.Close()

	var items []session.SyncItem
	for rows.Next() {
		var item session.SyncItem
		var createdNano int64

		err := rows.Scan(
			&item.ID, &item.TableName, &item.RecordID, &item.Operation,
			&item.Attempts, &createdNano, &item.LastError,
		)
		if err != nil {
			return nil, translateErr(err, "scanning sync item row")
		}

		item.CreatedAt = time.Unix(0, createdNano)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, translateErr(err, "iterating sync items")
	}

	return items, nil
}

func (r *SyncQueueRepo) MarkSynced(ctx context.Context, id string) error {
	query := `DELETE FROM sync_queue WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return translateErr(err, "marking item synced")
	}

	return checkRowsAffected(result, session.ErrNotFound, "marking item synced")
}

func (r *SyncQueueRepo) MarkFailed(ctx context.Context, id string, reason string) error {
	query := `
		UPDATE sync_queue 
		SET attempts = attempts + 1, last_error = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, reason, id)
	if err != nil {
		return translateErr(err, "marking item failed")
	}

	return checkRowsAffected(result, session.ErrNotFound, "marking item failed")
}

func (r *SyncQueueRepo) PendingCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM sync_queue`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, translateErr(err, "counting pending sync items")
	}

	return count, nil
}
