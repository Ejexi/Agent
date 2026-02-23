package ports

import "context"

// MemoryPort defines a generic interface for external memory storage.
//
// Deprecated: Prefer the purpose-specific ports instead:
//   - MetadataDB  (metadatadb.go) — PostgreSQL for vulnerability metadata
//   - LogDB       (logdb.go)      — Elasticsearch for raw scan logs
//   - VectorDB    (vectordb.go)   — pgvector for embeddings
//
// MemoryPort remains available as a fallback for simple key-value use cases.
type MemoryPort interface {
	Save(ctx context.Context, key string, data interface{}) error
	Load(ctx context.Context, key string) (interface{}, error)
}
