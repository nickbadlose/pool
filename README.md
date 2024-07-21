# Pool Package

The pool package allows a use to easily spin up a worker pool with minimal effort, it is designed to be used in 
processes where a user often needs to use worker pools to handle multiple jobs concurrently for speed. See [usage](#usage).

We have [benchmarked](#benchmarks) the package against just using the standard go lib, the performance difference is minimal. However, 
there are, of course many pipeline patterns to benchmark, so feel free to test it out against them, this is just a
common example. You should only be using the Dispatcher pipeline if you feel it can help you handle the complexities 
of concurrency, rather than writing your own.

## Usage

The Dispatcher returned from the WorkerPool.Start(ctx) method, can be used as either a singular worker pool or as part 
of a pipeline.

### Pipeline

```go
package mypackage

import (
    "context"
    "errors"
    "time"
	
    "github.com/nickbadlose/pool"
)

type worker struct {
	i int
}

type workerRes struct {
	i   int
	err error
}

func (w *worker) Work(context.Context) any {
	// perform some work, return error cases.
	return &workerRes{i: w.i, err: nil}
}

// pipeline increments the workers value by 1 at each stage of the pipeline.
//
// It returns the sum of all the jobs final value.
func pipeline(ctx context.Context, jobs int) (int, error) {
    wp := pool.NewWorkerPool()
    
    d := wp.Start(ctx)
    go func() {
        defer d.Done()
        for i := 0; i < jobs; i++ {
            err := d.Dispatch(&worker{i: 1})
            if err != nil {
                return
            }
        }
    }()
    
    d2 := wp.Start(ctx)
    go func() {
        defer d2.Done()
        for r := range d.Receive() {
            w, err := workerHandler(r)
            if err != nil {
                d.SetErr(err)
                return
            }
            err = d2.Dispatch(w)
            if err != nil {
                return
            }
        }
    }()
    
    d3 := wp.Start(ctx)
    go func() {
        defer d3.Done()
        for r := range d2.Receive() {
            w, err := workerHandler(r)
            if err != nil {
                d.SetErr(err)
                return
            }
            err = d3.Dispatch(w)
            if err != nil {
                return
            }
        }
    }()
    
    total := 0
    for r := range d3.Receive() {
        wr, ok := r.(*workerRes)
        if !ok {
            d3.SetErr(errors.New("not a workerRes"))
            break
        }
        if wr.err != nil {
            d3.SetErr(wr.err)
            break
        }
        
        total += wr.i
    }
    
    return total, wp.Err(d, d2, d3)
}

func workerHandler(w any) (*worker, error) {
    wr, ok := w.(*workerRes)
    if !ok {
        return nil, errors.New("not a workerRes")
    }
    if wr.err != nil {
        return nil, wr.err
    }
    return &worker{i: wr.i + 1}, nil
}
```

### Single Stage

```go
package main

import (
	"context"
	"log"

	"github.com/nickbadlose/pool"
)

func main() {
    ctx, cancel := context.WithCancel()
    defer cancel()
    
    jobs := make([]worker, 10)
    
    wp := pool.NewWorkerPool()
    d := wp.Start(ctx, len(jobs))
    go func() {
        defer d.Done()
        for _, j := range jobs {
            err := d.Dispatch(j)
            if err != nil {
                break
            }
        }
    }()
    
    for r := range d.Receive() {
        log.Println(r)
        // handle response
    }
    
    if d.Err() != nil {
        // handle error
    }
    
    // continue	
}
```

### Buffered vs Unbuffered

TODO check the performance cost after benchmarking different pattern.

A user can specify the number of jobs they will be dispatching beforehand, in which case the dispatcher will use a 
buffered channel. This can be useful for certain concurrency patterns, however, this comes at a [performance cost](#benchmarks).

- Buffered: `wp.Start(ctx, jobs)`
- Unbuffered: `wp.Start(ctx)`

Buffered example:

```go
package main

import (
	"context"
	"log"
	"sync"

	"github.com/nickbadlose/pool"
)

func main() {
    ctx, cancel := context.WithCancel()
    defer cancel()
    
    jobs := make([]*worker, 10)
    mu := sync.Mutex{}
    wg := sync.WaitGroup{}
    total := 0
    
    wp := pool.NewWorkerPool()
    d := wp.Start(ctx, len(jobs))
    wg.Add(1)
    go func() {
        defer wg.Done()
        for r := range d.Receive() {
            wr, ok := r.(*workerRes)
            if !ok {
                d.SetErr(errors.New("not a workerRes"))
                return
            }
            if wr.err != nil {
                d.SetErr(wr.err)
                return
            }
    
            mu.Lock()
            total += wr.i
            mu.Unlock()
        }
    }()
    
    for _, j := range jobs {
        err := d.Dispatch(j)
        if err != nil {
            break
        }
    }
    d.Done()
    wg.Wait()
    
    if d.Err() != nil {
        // handle error
    }
    
    log.Println(total)
    
    // continue	
}
```

## Testing

To test generic functions, without fuzz and benchmarks, run:

```
go test ./... -count=1 -short
```

### Benchmarks

To run benchmarks, run:

```
go test -bench=<functionRegex> -benchtime=10s -benchmem
```

To simulate a general step for benchmarking I have marshalled and unmarshalled a `map[string]string` at each stage 
of the pipeline, the map has 100 elements with each key and value a 100 character string. 

There are 3 types of pipelines included in the benchmarks:
1. Generic pipeline that doesn't use the dispatcher, this uses unbuffered channels. See `testGenericPipeline` function.
2. Buffered dispatcher pipeline.
3. Unbuffered dispatcher pipeline. 

Here are the results, in the `-bench` flag of the test command, J represents the number of jobs and W represents the 
number of workers processing them:

```
go test -bench=J1000W10HW -benchtime=10s -benchmem
goos: darwin
goarch: arm64
pkg: github.com/nickbadlose/pool
BenchmarkGenericJ1000W10HW-16                122          98186250 ns/op        200586394 B/op   1558052 allocs/op
BenchmarkUnbufferedJ1000W10HW-16             124          95572461 ns/op        201097824 B/op   1570006 allocs/op
BenchmarkBufferedJ1000W10HW-16               100         105500246 ns/op        196339596 B/op   1568570 allocs/op
PASS
ok      github.com/nickbadlose/pool     55.474s
```

```
go test -bench=J100W10HW -benchtime=10s -benchmem 
goos: darwin
goarch: arm64
pkg: github.com/nickbadlose/pool
BenchmarkGenericJ100W10HW-16                 934          12638872 ns/op        22378933 B/op     156605 allocs/op
BenchmarkUnbufferedJ100W10HW-16              949          12531544 ns/op        22436337 B/op     157809 allocs/op
BenchmarkBufferedJ100W10HW-16                592          20621393 ns/op        19654961 B/op     157014 allocs/op
PASS
ok      github.com/nickbadlose/pool     41.998s

```

```
go test -bench=J10W5HW -benchtime=10s -benchmem
goos: darwin
goarch: arm64
pkg: github.com/nickbadlose/pool
BenchmarkGenericJ10W5HW-16                  6326           1698978 ns/op         2358515 B/op      15731 allocs/op
BenchmarkUnbufferedJ10W5HW-16               7316           1721392 ns/op         2399230 B/op      15869 allocs/op
BenchmarkBufferedJ10W5HW-16                 3729           3073782 ns/op         1973799 B/op      15796 allocs/op
PASS
ok      github.com/nickbadlose/pool     36.926s

```

As you can see the buffered dispatcher is slightly slower than the unbuffered dispatcher, whereas the generic and 
unbuffered dispatcher perform at a similar level.

### Fuzz

TODO docs on fuzz tests

`FuzzDispatcherPipeline`

## Improvements / TODOs

- Fuzz test pipelines using the wait method.
- Should we allow users to specify the number of workers at the dispatcher level, maybe take in ...Options?
- Add a WithLock method which takes func for setting things with a lock, could have a read method too which wraps 
  in a rLock. This should use a different RWMutex to the one used internally. e.g:
    ```go
    package main
    
    import (
    "context"
    "log"
    "sync"
    
        "github.com/nickbadlose/pool"
    )
    
    func main() {
    ctx, cancel := context.WithCancel()
    defer cancel()
    
        jobs := make([]*worker, 10)
        wg := sync.WaitGroup{}
        total := 0
        
        wp := pool.NewWorkerPool()
        d := wp.Start(ctx, len(jobs))
        wg.Add(1)
        go func() {
            defer wg.Done()
            for r := range d.Receive() {
                wr, ok := r.(*workerRes)
                if !ok {
                    d.SetErr(errors.New("not a workerRes"))
                    return
                }
                if wr.err != nil {
                    d.SetErr(wr.err)
                    return
                }
        
                d.WithLock(func() {
                    total += wr.i
                })
            }
        }()
        
        for _, j := range jobs {
            err := d.Dispatch(j)
            if err != nil {
                break
            }
        }
        d.Done()
        wg.Wait()
        
        if d.Err() != nil {
            // handle error
        }
        
        log.Println(total)
        
        // continue	
    }
    ```
- Have Done, Add and Wait methods attached which increment and wait for an internal WG, should use a different one from 
  the internal one. 
- Could we have two different dispatchers, buffered and unbuffered, so we don't have all this overhead on every one 
  returned? We may not need mutexes and wait groups on buffered?
