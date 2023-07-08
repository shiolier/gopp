package gopp

import "context"

// Runner is a process of parallel processing.
type Runner[T any] interface {
	Run(context.Context) (T, error)
}

type runner[T any] struct {
	fn func(context.Context) (T, error)
}

func (r *runner[T]) Run(ctx context.Context) (T, error) {
	return r.fn(ctx)
}

// NewRunner converts func(context.Context) (T, error) to Runner
func NewRunner[T any](fn func(context.Context) (T, error)) Runner[T] {
	return &runner[T]{
		fn: fn,
	}
}
