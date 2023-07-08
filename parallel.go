// gopp is package restricts max num of parallel processing.
//
// Example
//
//	// Context
//	ctx, cancel := context.WithCancel(context.Background())
//	// can set a timeout for the entire process using context.WithTimeout
//	//ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
//	defer cancel()
//
//	// New
//	// process result is int (and error)
//	p := gopp.New[int](
//		// context
//		ctx,
//		// max num of parallel processing is 4
//		gopp.Procs(4),
//		// timeout for each process is 3s
//		gopp.RunnerTimeout(3*time.Second))
//
//	// Add Runner
//	r := gopp.NewRunner[int](func(ctx context.Context) (int, error) {
//		// heavy process
//		time.Sleep(time.Second)
//		return 123, nil
//	})
//	for i := 0; i < 100; i++ {
//		p.Add(r)
//	}
//
//	// Wait and receive results
//	ress := p.Wait()
//	for _, res := range ress {
//		if res.Err != nil {
//			fmt.Printf("Error: %v\n", res.Err)
//			continue
//		}
//		fmt.Println(res.Value)
//	}
//
//	// Don't reuse p, because it will not working properly.
//	// If you want to reuse it, please recreate it.
//	p = gopp.New[int](context.Background(), gopp.Procs(4))
package gopp

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Parallel[T any] interface {
	// Add adds Runner(s)
	Add(rs ...Runner[T]) error
	// Result returns channel for receiving Result
	Result() <-chan *Result[T]
	// Done returns channel that will be closed when all processing is done.
	Done() <-chan struct{}
	// Wait blocks until all processing to done, and returns all results.
	Wait() []*Result[T]
}

// parallel restricts max num of parallel processing.
//
// Don't reuse this, because it will not working properly.
type parallel[T any] struct {
	ctx    context.Context
	sem    chan struct{}
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
func New[T any](ctx context.Context, opts ...Option) Parallel[T] {
	o := new(option)
	for _, opt := range opts {
		opt(o)
	}
	if o.procs < 1 {
		o.procs = ProcsDefault
	}

	p := &parallel[T]{
		ctx:    ctx,
		sem:    make(chan struct{}, o.procs),
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

func (p *parallel[T]) loop() {
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

			select {
			case <-p.ctx.Done():
				p.ctxdone(p.ctx)
				return
			case p.sem <- struct{}{}:
			}
			defer func() {
				<-p.sem
			}()

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

func (p *parallel[T]) ctxdone(ctx context.Context) {
	var t T
	p.resch <- &Result[T]{
		Value: t,
		Err:   &ErrContextDone{context.Cause(ctx)},
	}
}

func (p *parallel[T]) alldone() {
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

// Add adds Runner(s)
func (p *parallel[T]) Add(rs ...Runner[T]) error {
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
func (p *parallel[T]) Result() <-chan *Result[T] {
	return p.resch
}

// Done returns channel that will be closed when all processing is done.
func (p *parallel[T]) Done() <-chan struct{} {
	p.once.Do(func() {
		go func() {
			p.wg.Wait()
			close(p.donech)
		}()
	})
	return p.donech
}

// Wait blocks until all processing to done, and returns all results.
func (p *parallel[T]) Wait() (ret []*Result[T]) {
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
