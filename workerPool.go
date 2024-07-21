package pool

import (
	"context"
	"sync"
	"time"
)

const (
	defaultWorkers     = 5
	defaultWorkTimeout = 2 * time.Minute
)

// WorkerPool allows us to Start worker pools with the given configuration.
type WorkerPool struct {
	*config
}

func (wp *WorkerPool) workers() int {
	if wp.config == nil {
		return minWorkers
	}
	return wp.config.workers
}

func (wp *WorkerPool) workTimeout() time.Duration {
	if wp.config == nil {
		return 0
	}
	return wp.config.workTimeout
}

func (wp *WorkerPool) poolTimeout() *time.Duration {
	if wp.config == nil {
		return nil
	}
	return wp.config.poolTimeout
}

// NewWorkerPool builds a new WorkerPool.
func NewWorkerPool(os ...Option) *WorkerPool {
	cfg := &config{workers: defaultWorkers, workTimeout: defaultWorkTimeout}
	for _, o := range os {
		o.apply(cfg)
	}

	return &WorkerPool{cfg}
}

// Start sets up a worker pool and returns a Dispatcher for interaction.
//
// The Dispatcher allows the user to dispatch any jobs they require doing down the worker channel,
// via the Dispatcher.Dispatch() method. Once all jobs have been dispatched, it is up to the user to signal that
// the channel can be closed, by calling the Done method.
//
// The user can start receiving any completed jobs as early as they like, via the basic Dispatcher.Receive() method.
// Once the user has finished receiving jobs, a call to Dispatcher.Err() should be made, to check if any error was
// encountered.
//
// The worker chan will be either buffered or unbuffered depending on whether any list of job numbers are supplied. The
// user should handle dispatching to either channel appropriately to avoid a deadlock.
func (wp *WorkerPool) Start(ctx context.Context, jobs ...int) *Dispatcher {
	totalJobs := 0
	for _, j := range jobs {
		totalJobs += j
	}

	dr := &Dispatcher{
		ctx:     ctx,
		rcv:     make(chan any, totalJobs),
		wc:      make(chan Worker, totalJobs),
		timeout: wp.workTimeout(),
		closed:  false,
		wg:      sync.WaitGroup{},
		mu:      sync.RWMutex{},
	}

	wp.do(dr)

	return dr
}

// Err returns the last error received by any of the given dispatchers.
//
// Helpful when using a pipeline of dispatchers, and you only want to check for errors at the end, rather than at each
// stage.
func (wp *WorkerPool) Err(ds ...*Dispatcher) error {
	for _, d := range ds {
		if err := d.Err(); err != nil {
			return err
		}
	}
	return nil
}
