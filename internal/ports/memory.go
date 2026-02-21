package ports

import "context"

// MemoryPort defines the interface for external memory storage.
type MemoryPort interface {
	Save(ctx context.Context, key string, data interface{}) error
	Load(ctx context.Context, key string) (interface{}, error)
}
