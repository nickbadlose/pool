# Pool Package

The pool package allows a use to easily spin up a worker pool with minimal effort, it is designed to be used in 
processes where a user often needs to use worker pools to handle multiple jobs concurrently for speed. It should 
abstract away some of the nuances of handling concurrency safely, making it easy to prevent race conditions and 
deadlocks. See [usage](#usage).

We have [benchmarked](#benchmarks) the package against just using the standard go lib, the performance difference is 
minimal. However, there are, of course many pipeline patterns to benchmark, so feel free to test it out against them, 
this is just a common example. You should only be using the Dispatcher pipeline if you feel it can help you handle 
the complexities of concurrency, rather than writing your own.

## Usage

The Dispatcher returned from the WorkerPool.Start(ctx) method, can be used as either a [singular](#single-stage) worker 
pool or as part of a [pipeline](#pipeline).

### Worker Interface

To use the package, any item that is dispatched must implement the `Worker` interface. This method is the one that will 
be run in the worker pool and therefore should always handle the most time-consuming tasks, such as HTTP request, 
marshalling and unmarshalling of data etc.

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
go test -bench=<functionRegex> -benchtime=10s -benchmem -count=1
```

To simulate a general step for benchmarking I have marshalled and unmarshalled a `map[string]string` at each stage 
of the pipeline, the map has 100 elements with each key and value a 100 character string. 

There are 3 types of pipelines included in the benchmarks:
1. Generic pipeline that doesn't use the dispatcher, unbuffered. See `testGenericPipeline6Steps` function.
2. Generic pipeline that doesn't use the dispatcher, unbuffered. It merges a receiver and dispatcher steps into one 
   step. See `testGenericPipeline3Steps` function.
3. Buffered dispatcher pipeline.
4. Unbuffered dispatcher pipeline. 

Here are the results, in the `-bench` flag of the test command, J represents the number of jobs and W represents the 
number of workers processing them:

```
go test -bench=J1000W10 -benchtime=10s -benchmem
goos: darwin
goarch: arm64
pkg: github.com/nickbadlose/pool
BenchmarkGeneric6StepsJ1000W10-16            254          47115684 ns/op        197806557 B/op   1557339 allocs/op
BenchmarkGeneric3StepsJ1000W10-16            258          46560093 ns/op        197333406 B/op   1557192 allocs/op
BenchmarkUnbufferedJ1000W10-16               248          47843100 ns/op        198677357 B/op   1569397 allocs/op
BenchmarkBufferedJ1000W10-16                 254          66653843 ns/op        194317184 B/op   1567975 allocs/op
PASS
ok      github.com/nickbadlose/pool     73.379s
```

```
go test -bench=J100W10 -benchtime=10s -benchmem 
goos: darwin
goarch: arm64
pkg: github.com/nickbadlose/pool
BenchmarkGeneric6StepsJ100W10-16            1572           7675767 ns/op        21415472 B/op     156338 allocs/op
BenchmarkGeneric3StepsJ100W10-16            1569           7689075 ns/op        21456664 B/op     156343 allocs/op
BenchmarkUnbufferedJ100W10-16               1455           8177907 ns/op        21786647 B/op     157639 allocs/op
BenchmarkBufferedJ100W10-16                 1056          14547064 ns/op        19482136 B/op     156957 allocs/op
PASS
ok      github.com/nickbadlose/pool     56.395s
```

```
go test -bench=J10W5 -benchtime=10s -benchmem 
goos: darwin
goarch: arm64
pkg: github.com/nickbadlose/pool
BenchmarkGeneric6StepsJ10W5-16              8292           1390729 ns/op         2494266 B/op      15772 allocs/op
BenchmarkGeneric3StepsJ10W5-16              9121           1370628 ns/op         2467171 B/op      15760 allocs/op
BenchmarkUnbufferedJ10W5-16                 8239           1476730 ns/op         2546269 B/op      15914 allocs/op
BenchmarkBufferedJ10W5-16                   4081           2481791 ns/op         1968424 B/op      15790 allocs/op
PASS
ok      github.com/nickbadlose/pool     48.525s
```

As you can see the buffered dispatcher is slightly slower than the unbuffered dispatcher, whereas the generic and 
unbuffered dispatcher perform at a similar level.

### Fuzz

The `testDispatcherPipeline` flow has been fuzz tested with different worker pool configurations to find any bugs and 
fixed where necessary. See `FuzzDispatcherPipeline` for more information.

### Profiling

To run tests with profiling output to be read by `pprof`, run:

```bash
go test -bench=J100W10 -benchtime=2s -test.cpuprofile=test.prof
```

## Improvements / TODOs

- Semantic release
- Linting
- CICD
- Benchmarks have revealed bottlenecks when we use the `Receive` method as it isn't in a pool. Look into improving 
  this. Or should we just document to keep `Receive` loops light, and state to use a worker pool to receive if heavy.
- Fuzz test different pipeline methods, including buffered pipelines using wait groups and mutexes etc.
- Run profiling to see why there is a difference in the generic pipeline performances.
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
