package gopp

// Result is result of [Runner].
type Result[T any] struct {
	Value T
	Err   error
}
