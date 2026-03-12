package taskengine

import (
	"context"
	"github.com/SecDuckOps/agent/internal/domain"
)

// TaskHandler defines the signature for a function that processes an OSTask.
type TaskHandler func(ctx context.Context, task *domain.OSTask) domain.OSTaskResult

// TaskMiddleware is a function that wraps a TaskHandler to provide cross-cutting concerns.
type TaskMiddleware func(next TaskHandler) TaskHandler

// ChainMiddleware combines multiple middlewares into a single TaskHandler.
func ChainMiddleware(handler TaskHandler, mw ...TaskMiddleware) TaskHandler {
	for i := len(mw) - 1; i >= 0; i-- {
		handler = mw[i](handler)
	}
	return handler
}
