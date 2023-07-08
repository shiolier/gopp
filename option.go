package gopp

import (
	"runtime"
	"time"
)

// option is option for [New].
type option struct {
	procs    int
	timeout  time.Duration
	reschbuf int
}

// Option is option for [New].
type Option func(*option)

// Procs specifies max num of parallel processing.
func Procs(n int) Option {
	return func(o *option) {
		o.procs = n
	}
}

// ProcsNumCPU sets [runtime.NumCPU] to [Procs].
func ProcsNumCPU() Option {
	return Procs(runtime.NumCPU())
}

// RunnerTimeout specifies timeout of [Runner].
//
// If 0 or less is specified, no timeout is set.
func RunnerTimeout(d time.Duration) Option {
	return func(o *option) {
		o.timeout = d
	}
}

// ResultChBuf specifies buffer of result channel.
//
// Deprecated: [Parallel.Done] fires before receiving all results.
func ResultChBuf(n int) Option {
	return func(o *option) {
		o.reschbuf = n
	}
}
