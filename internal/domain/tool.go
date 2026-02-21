package domain

import "context"

type Tool interface {

	Name() string

	Run(
		ctx context.Context,
		task Task,
	) (Result, error)
}