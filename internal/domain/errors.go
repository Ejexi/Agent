package domain

import "errors"

// Domain-level sentinel errors.
var (
	// ErrCyclicDependency is returned when DAG topological sort detects a cycle.
	ErrCyclicDependency = errors.New("cyclic dependency detected in execution plan")
)
