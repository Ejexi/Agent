package pgvector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/shared/types"

	_ "github.com/lib/pq" // PostgreSQL driver
	pgv "github.com/pgvector/pgvector-go"
)

// Adapter implements ports.VectorDB using PostgreSQL + pgvector extension.
// Used for semantic search, RAG retrieval, and storing vulnerability embeddings.
type Adapter struct {
	db *sql.DB
}

// Config holds connection parameters for the pgvector adapter.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DSN builds a PostgreSQL connection string.
func (c Config) DSN() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, sslMode,
	)
}

// NewAdapter creates a new pgvector adapter and verifies the connection.
func NewAdapter(cfg Config) (*Adapter, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "pgvector: failed to open connection")
	}

	// Connection pool tuning
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, types.Wrap(err, types.ErrCodeInternal, "pgvector: failed to ping")
	}

	return &Adapter{db: db}, nil
}

// Migrate enables the pgvector extension and creates the embeddings table.
func (a *Adapter) Migrate(ctx context.Context) error {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS vector`,
		`CREATE TABLE IF NOT EXISTS embeddings (
			id       TEXT PRIMARY KEY,
			content  TEXT NOT NULL,
			vector   vector(1536),
			source   TEXT NOT NULL,
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_embeddings_source ON embeddings(source)`,
	}

	for _, q := range queries {
		if _, err := a.db.ExecContext(ctx, q); err != nil {
			return types.Wrap(err, types.ErrCodeInternal, "pgvector: migration failed")
		}
	}

	return nil
}

// Upsert stores an embedding document, replacing it if the ID already exists.
func (a *Adapter) Upsert(ctx context.Context, doc ports.EmbeddingDocument) error {
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "pgvector: marshal metadata")
	}

	vec := pgv.NewVector(doc.Vector)

	_, err = a.db.ExecContext(ctx,
		`INSERT INTO embeddings (id, content, vector, source, metadata)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET
		   content = EXCLUDED.content,
		   vector = EXCLUDED.vector,
		   source = EXCLUDED.source,
		   metadata = EXCLUDED.metadata`,
		doc.ID, doc.Content, vec, doc.Source, metadataJSON,
	)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "pgvector: upsert embedding")
	}

	return nil
}

// Search performs a cosine similarity search against stored embeddings.
func (a *Adapter) Search(ctx context.Context, query ports.VectorSearchQuery) ([]ports.EmbeddingResult, error) {
	topK := query.TopK
	if topK <= 0 {
		topK = 10
	}

	vec := pgv.NewVector(query.Vector)

	// Build WHERE clause from metadata filters
	whereClauses := []string{}
	args := []interface{}{vec, topK}
	argIdx := 3

	for key, value := range query.Filter {
		whereClauses = append(whereClauses,
			fmt.Sprintf("metadata->>$%d = $%d", argIdx, argIdx+1),
		)
		args = append(args, key, value)
		argIdx += 2
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Use cosine distance operator (<=>), lower = more similar
	// Convert to similarity score: 1 - cosine_distance
	sqlQuery := fmt.Sprintf(
		`SELECT id, content, vector, source, metadata,
		        1 - (vector <=> $1) AS score
		 FROM embeddings
		 %s
		 ORDER BY vector <=> $1
		 LIMIT $2`,
		whereSQL,
	)

	rows, err := a.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "pgvector: search")
	}
	defer rows.Close()

	var results []ports.EmbeddingResult
	for rows.Next() {
		var doc ports.EmbeddingDocument
		var vec pgv.Vector
		var metadataJSON []byte
		var score float32

		if err := rows.Scan(&doc.ID, &doc.Content, &vec, &doc.Source, &metadataJSON, &score); err != nil {
			return nil, types.Wrap(err, types.ErrCodeInternal, "pgvector: scan result row")
		}

		doc.Vector = vec.Slice()
		json.Unmarshal(metadataJSON, &doc.Metadata)

		// Apply threshold filter if specified
		if query.Threshold > 0 && score < query.Threshold {
			continue
		}

		results = append(results, ports.EmbeddingResult{
			Document: doc,
			Score:    score,
		})
	}

	return results, rows.Err()
}

// Delete removes an embedding document by its ID.
func (a *Adapter) Delete(ctx context.Context, id string) error {
	_, err := a.db.ExecContext(ctx,
		`DELETE FROM embeddings WHERE id = $1`, id,
	)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "pgvector: delete")
	}
	return nil
}

// DeleteBySource removes all embeddings originating from a specific source.
func (a *Adapter) DeleteBySource(ctx context.Context, source string) error {
	_, err := a.db.ExecContext(ctx,
		`DELETE FROM embeddings WHERE source = $1`, source,
	)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "pgvector: delete by source")
	}
	return nil
}

// Close gracefully shuts down the database connection pool.
func (a *Adapter) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}
