package ports

import "context"

// VectorDB defines the interface for the embedding store (pgvector).
// Used for semantic search, RAG retrieval, and storing vulnerability embeddings.
type VectorDB interface {
	// Upsert stores an embedding vector with its associated metadata.
	// If a document with the same ID exists, it is replaced.
	Upsert(ctx context.Context, doc EmbeddingDocument) error

	// Search performs a similarity (nearest-neighbor) search against stored embeddings.
	Search(ctx context.Context, query VectorSearchQuery) ([]EmbeddingResult, error)

	// Delete removes an embedding document by its ID.
	Delete(ctx context.Context, id string) error

	// DeleteBySource removes all embeddings originating from a specific source
	// (e.g., all embeddings from a particular scan).
	DeleteBySource(ctx context.Context, source string) error

	// Close gracefully shuts down the connection.
	Close() error
}

// EmbeddingDocument represents a single document stored in the vector database.
type EmbeddingDocument struct {
	ID       string            `json:"id"`                 // Unique document identifier
	Content  string            `json:"content"`            // Original text content
	Vector   []float32         `json:"vector"`             // Embedding vector (dimension depends on model)
	Source   string            `json:"source"`             // Origin reference (e.g., scan ID, file path)
	Metadata map[string]string `json:"metadata,omitempty"` // Arbitrary key-value pairs for filtering
}

// VectorSearchQuery provides parameters for a similarity search.
type VectorSearchQuery struct {
	Vector    []float32         `json:"vector"`              // Query embedding vector
	TopK      int               `json:"top_k"`               // Number of nearest neighbors to return
	Threshold float32           `json:"threshold,omitempty"` // Minimum similarity score (0.0–1.0)
	Filter    map[string]string `json:"filter,omitempty"`    // Metadata-based pre-filtering
}

// EmbeddingResult represents a single result from a similarity search.
type EmbeddingResult struct {
	Document EmbeddingDocument `json:"document"`
	Score    float32           `json:"score"` // Similarity score (higher = more similar)
}
