# GOPP - Go parallel processing

[![Go Reference](https://pkg.go.dev/badge/github.com/shiolier/gopp.svg)](https://pkg.go.dev/github.com/shiolier/gopp)

GOPP is a Go library for restricting max num of parallel processing.

## Import

```go
import "github.com/shiolier/gopp"
```

## Example

```go

import (
	"context"
	"fmt"
	"time"

	"github.com/shiolier/gopp"
)

func Example() {
	ctx, cancel := context.WithCancel(context.Background())
	// can set a timeout for the entire process using context.WithTimeout
	//ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()

	// New
	// process result is int (and error)
	p := gopp.New[int](
		// context
		ctx,
		// max num of parallel processing is 4
		gopp.Procs(4),
		// timeout for each process is 3s
		gopp.RunnerTimeout(3*time.Second))

	// Add Runner
	rs := make([]gopp.Runner[int], 10)
	for i := 0; i < 10; i++ {
		rs[i] = &SampleRunner{i}
		// can also add each time.
		//p.Add(&SampleRunner{i})
	}
	p.Add(rs...)

	// Receive the result
loop: // label for break
	for {
		select {
		case res := <-p.Result(): // receive the result
			// error check
			if res.Err != nil {
				fmt.Printf("Error: %v\n", res.Err)
				// If you want to stop other processing, please call cancel.
				cancel()
				// But you should use continue instead of break to receive all results (including context.Canceled error).
				// Otherwise the sender will block and cause a goroutine leak.
				continue
			}
			fmt.Println(res.Value)

			// can also add Runner here
			//p.Add(&SampleRunner{10})
		case <-p.Done(): // all processing done
			// break for
			break loop
		}
	}
	// or
	// ress := p.Wait()
	// for _, res := range ress {
	// 	// omit
	// }

	// p cannot be reused, because it will not working properly.
	// If you want to use it again, please recreate it in New.
	//p = gopp.New[int](ctx, gopp.ProcsNumCPU())

	// Unordered output:
	// 0
	// 1
	// 4
	// 9
	// 16
	// 25
	// 36
	// 49
	// 64
	// 81
}

type SampleRunner struct {
	n int
}

func (s *SampleRunner) Run(ctx context.Context) (int, error) {
	// heavy process
	time.Sleep(time.Second)
	return s.n * s.n, nil
}

```

## License

[MIT License](LICENSE)
