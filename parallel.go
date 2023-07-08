// gopp is package restricts max num of parallel processing.
package gopp

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sync/semaphore"
)

// Parallel restricts max num of parallel processing.
//
// Don't initialize this struct outside of [New].
//
// Don't reuse this, because it will not working properly.
type Parallel[T any] struct {
	ctx    context.Context
	sem    *semaphore.Weighted
	wg     sync.WaitGroup
	addch  chan Runner[T]
	resch  chan *Result[T]
	donech chan struct{}
	once   sync.Once
	opt    *option
}

// default max num of parallel processing
var ProcsDefault = 1

// New returns Parallel.
func New[T any](ctx context.Context, opts ...Option) *Parallel[T] {
	o := new(option)
	for _, opt := range opts {
		opt(o)
	}
	if o.procs < 1 {
		o.procs = ProcsDefault
	}

	p := &Parallel[T]{
		ctx:    ctx,
		sem:    semaphore.NewWeighted(int64(o.procs)),
		wg:     sync.WaitGroup{},
		addch:  make(chan Runner[T]),
		resch:  make(chan *Result[T], o.reschbuf),
		donech: make(chan struct{}, 1),
		once:   sync.Once{},
		opt:    o,
	}
	// start loop
	go p.loop()
	return p
}

func (p *Parallel[T]) loop() {
	for {
		var r Runner[T]
		// receive ctx.Done or Runner
		select {
		case <-p.ctx.Done():
			p.alldone()
			return
		case r = <-p.addch:
		}

		go func() {
			defer p.wg.Done()

			if r == nil {
				var t T
				p.resch <- &Result[T]{
					Value: t,
					Err:   errors.New("runner is nil"),
				}
				return
			}

			if err := p.sem.Acquire(p.ctx, 1); err != nil {
				p.resch <- &Result[T]{
					Err: fmt.Errorf("failed to acquire from semaphore: %w", err),
				}
				return
			}
			defer p.sem.Release(1)

			// context check
			select {
			case <-p.ctx.Done():
				p.ctxdone(p.ctx)
				return
			default:
			}

			// timeout
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)
			if p.opt != nil && p.opt.timeout > 0 {
				ctx, cancel = context.WithTimeout(p.ctx, p.opt.timeout)
			} else {
				ctx, cancel = context.WithCancel(p.ctx)
			}
			defer cancel()

			t, err := r.Run(ctx)
			p.resch <- &Result[T]{
				Value: t,
				Err:   err,
			}
		}()
	}
}

func (p *Parallel[T]) ctxdone(ctx context.Context) {
	var t T
	p.resch <- &Result[T]{
		Value: t,
		Err:   &ErrContextDone{context.Cause(ctx)},
	}
}

func (p *Parallel[T]) alldone() {
	// receive all Runners
	for b := true; b; {
		select {
		case <-p.addch:
			// send Result
			p.ctxdone(p.ctx)
			p.wg.Done()
		case <-p.Done():
			b = false
		}
	}
}

// Add adds Runner(s) to Parallel
func (p *Parallel[T]) Add(rs ...Runner[T]) error {
	// context check
	select {
	case <-p.ctx.Done():
		return &ErrContextDone{context.Cause(p.ctx)}
	default:
	}

	p.wg.Add(len(rs))

	// send Runner async
	/*
		for _, r := range rs {
			go func(r Runner[T]) {
				p.addch <- r
			}(r)
		}
	*/
	go func() {
		for _, r := range rs {
			p.addch <- r
		}
	}()

	return nil
}

// Result returns channel for receiving Result.
func (p *Parallel[T]) Result() <-chan *Result[T] {
	return p.resch
}

// Done returns channel that will be closed when all processing is done.
func (p *Parallel[T]) Done() <-chan struct{} {
	p.once.Do(func() {
		go func() {
			p.wg.Wait()
			close(p.donech)
		}()
	})
	return p.donech
}

// Wait blocks until all processing to done, and returns all results.
func (p *Parallel[T]) Wait() (ret []*Result[T]) {
	for {
		select {
		case res := <-p.Result():
			ret = append(ret, res)
		case <-p.Done():
			return ret
		}
	}
}

// ErrContextDone is error with [context.Done].
type ErrContextDone struct {
	// Err is the return value of context.Cause(ctx)
	Err error
}

func (e *ErrContextDone) Error() string {
	return fmt.Sprintf("context done: %v", e.Err)
}

// Unwrap returns e.Err that is context.Cause(ctx).
func (e *ErrContextDone) Unwrap() error {
	return e.Err
}
